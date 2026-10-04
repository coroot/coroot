package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	mrand "math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"github.com/google/pprof/profile"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	colllogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

var (
	listenAddr     = getenv("LISTEN", "0.0.0.0:4000")
	corootOTLP     = ensureTraces(getenv("COROOT_OTLP_ENDPOINT", "http://coroot:8080/v1/traces"))
	corootLogsURL  = ensureLogs(getenv("COROOT_LOGS_URL", "http://coroot:8080/v1/logs"))
	apiKey         = getenv("COROOT_API_KEY", "agent-local-key-0000000000000001")
	serviceName    = getenv("SERVICE_NAME", "demo-api")
	serviceVersion = getenv("SERVICE_VERSION", "1.0.0")
	machineID      = getenv("DEMO_MACHINE_ID", "rumlabdemomachine000000000000001")
	systemUUID     = getenv("DEMO_SYSTEM_UUID", "rumlabdemosystemuuid0000000000001")
	containerID    = getenv("DEMO_CONTAINER_ID", "/docker/"+serviceName)
	hostname       = getenv("DEMO_HOSTNAME", "rum-lab-node")
	instanceName   = getenv("DEMO_INSTANCE", lastSeg(containerID, serviceName))
	metricsURL     = strings.TrimRight(getenv("COROOT_METRICS_URL", "http://coroot:8080/v1/metrics"), "/")
	profilesURL    = strings.TrimRight(getenv("COROOT_PROFILES_URL", "http://coroot:8080/v1/profiles"), "/")
	metricsEvery   = getenvF("METRICS_INTERVAL_SEC", 10)
	profilesEvery  = getenvF("PROFILES_INTERVAL_SEC", 45)
	databaseURL    = getenv("DATABASE_URL", "postgresql://shop:shop@demo-postgres:5432/shop")
	pgHost         = getenv("PG_HOST", "demo-postgres")
	pgPort         = getenv("PG_PORT", "5432")
	redisURL       = getenv("REDIS_URL", "redis://demo-valkey:6379/0")
	valkeyHost     = getenv("VALKEY_HOST", "demo-valkey")
	valkeyPort     = getenv("VALKEY_PORT", "6379")
	appRole        = strings.ToLower(strings.TrimSpace(getenv("APP_ROLE", "all")))
	cacheTTL       = time.Duration(getenvI("PRODUCTS_CACHE_TTL", 30)) * time.Second
	origins        = splitSet(getenv("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"))

	pgPool     *pgxpool.Pool
	rdb        *redis.Client
	pgAddr     = pgHost + ":" + pgPort
	valkeyAddr = valkeyHost + ":" + valkeyPort

	startAt   = time.Now()
	cpuAcc    float64
	cpuMu     sync.Mutex
	connectPG atomic.Uint64
	activePG  atomic.Int64
	connectVK atomic.Uint64
	activeVK  atomic.Int64

	profilePrev = map[string]map[uint64]int64{}
	profileMu   sync.Mutex
)

func main() {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(10000)
	runtime.MemProfileRate = 512 * 1024

	ctx := context.Background()
	var err error
	pgPool, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Printf("postgres: %v", err)
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal(err)
	}
	rdb = redis.NewClient(opt)
	resolveAddrs()
	if appRole == "portal" || appRole == "all" {
		ensureTicketsSchema(ctx)
	}

	go metricsLoop()
	go profilesLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/metrics", handleMetricsText)
	mux.HandleFunc("/api/products", cors(handleProducts))
	mux.HandleFunc("/api/checkout", cors(handleCheckout))
	mux.HandleFunc("/api/tickets", cors(handleTickets))
	mux.HandleFunc("/api/status", cors(handlePortalStatus))
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	log.Printf("[demo-api] listen=%s role=%s service=%s container=%s logs→%s profiles→%s",
		listenAddr, appRole, serviceName, containerID, corootLogsURL, profilesURL)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "service": serviceName, "role": appRole, "instance": instanceName})
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	traceID, parent := parseTP(r.Header.Get("traceparent"))
	spanID := newID(16)
	delay := applyDemoDelay(r)
	if code, msg := demoForceError(r); code > 0 {
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, code, map[string]any{"ok": false, "error": msg})
		go finishRequest(r, traceID, parent, spanID, start, code, msg, nil, delay)
		return
	}
	if appRole != "all" && appRole != "catalog" {
		writeJSON(w, 404, map[string]any{"ok": false, "error": "use_demo_api"})
		go finishRequest(r, traceID, parent, spanID, start, 404, "not found", nil, delay)
		return
	}
	items, err := fetchProducts(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
		go finishRequest(r, traceID, parent, spanID, start, 500, err.Error(), nil, delay)
		return
	}
	busy(len(items))
	end := time.Now()
	children := []span{
		mkSpan("GET/SETEX products cache + SELECT", 3, traceID, spanID, start, end, map[string]string{"db.system": "redis", "peer.service": "demo-valkey", "server.address": valkeyHost}, map[string]int64{"server.port": int64(atoi(valkeyPort)), "db.response.returned_rows": int64(len(items))}, 1, ""),
		mkSpan("GET products:all", 3, traceID, spanID, start, end, map[string]string{"db.system": "redis", "db.operation": "GET", "peer.service": "demo-valkey", "server.address": valkeyHost}, map[string]int64{"server.port": int64(atoi(valkeyPort))}, 1, ""),
		mkSpan("SELECT products", 3, traceID, spanID, start, end, map[string]string{"db.system": "postgresql", "db.operation": "SELECT", "peer.service": "demo-postgres", "server.address": pgHost, "db.name": "shop"}, map[string]int64{"server.port": int64(atoi(pgPort)), "db.response.returned_rows": int64(len(items))}, 1, ""),
	}
	w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
	writeJSON(w, 200, map[string]any{"items": items, "source": serviceName})
	go finishRequest(r, traceID, parent, spanID, start, 200, "", children, delay)
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	traceID, parent := parseTP(r.Header.Get("traceparent"))
	spanID := newID(16)
	delay := applyDemoDelay(r)
	if code, msg := demoForceError(r); code > 0 {
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, code, map[string]any{"ok": false, "error": msg})
		go finishRequest(r, traceID, parent, spanID, start, code, msg, nil, delay)
		return
	}
	if appRole != "all" && appRole != "checkout" {
		writeJSON(w, 404, map[string]any{"ok": false, "error": "use_demo_checkout"})
		go finishRequest(r, traceID, parent, spanID, start, 404, "not found", nil, delay)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	var children []span
	t0 := time.Now()
	n, _ := incrCheckout(r.Context())
	children = append(children, mkSpan("INCR checkout:count", 3, traceID, spanID, t0, time.Now(), map[string]string{"db.system": "redis", "db.operation": "INCR", "peer.service": "demo-valkey", "server.address": valkeyHost}, map[string]int64{"server.port": int64(atoi(valkeyPort)), "checkout.count": n}, 1, ""))

	payStart := time.Now()
	time.Sleep(time.Duration(80+mrand.Intn(120)) * time.Millisecond)
	fail := asBool(payload["fail"]) || r.Header.Get("X-Demo-Fail") == "1"
	payEnd := time.Now()
	if fail {
		children = append(children, mkSpan("POST /payments/charge", 3, traceID, spanID, payStart, payEnd, map[string]string{"http.method": "POST", "http.url": "http://payments.internal/charge", "server.address": "payments", "error.type": "PaymentDeclined"}, map[string]int64{"http.status_code": 402}, 2, "declined"))
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, 402, map[string]any{"ok": false, "error": "payment_declined"})
		go finishRequest(r, traceID, parent, spanID, start, 402, "payment declined", children, delay)
		return
	}
	orderID := fmt.Sprintf("ORD-%d", 10000+mrand.Intn(90000))
	total := asInt(payload["total"], 218)
	email, _ := payload["email"].(string)
	if email == "" {
		email = "demo@example.com"
	}
	dbStart := time.Now()
	_ = insertOrder(r.Context(), orderID, total, email)
	dbEnd := time.Now()
	children = append(children,
		mkSpan("INSERT orders", 3, traceID, spanID, dbStart, dbEnd, map[string]string{"db.system": "postgresql", "db.operation": "INSERT", "peer.service": "demo-postgres", "server.address": pgHost, "db.name": "shop", "order.id": orderID}, map[string]int64{"server.port": int64(atoi(pgPort))}, 1, ""),
		mkSpan("POST /payments/charge", 3, traceID, spanID, payStart, payEnd, map[string]string{"http.method": "POST", "http.url": "http://payments.internal/charge", "server.address": "payments"}, map[string]int64{"http.status_code": 200}, 1, ""),
	)
	busy(total)
	w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
	writeJSON(w, 200, map[string]any{"ok": true, "orderId": orderID, "total": total, "currency": "USD", "source": serviceName})
	go finishRequest(r, traceID, parent, spanID, start, 200, "", children, delay)
}

type product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func handleTickets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	traceID, parent := parseTP(r.Header.Get("traceparent"))
	spanID := newID(16)
	delay := applyDemoDelay(r)
	if code, msg := demoForceError(r); code > 0 {
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, code, map[string]any{"ok": false, "error": msg})
		go finishRequest(r, traceID, parent, spanID, start, code, msg, nil, delay)
		return
	}
	if appRole != "all" && appRole != "portal" {
		writeJSON(w, 404, map[string]any{"ok": false, "error": "use_demo_portal_api"})
		go finishRequest(r, traceID, parent, spanID, start, 404, "not found", nil, delay)
		return
	}
	switch r.Method {
	case http.MethodGet, "":
		items, err := fetchTickets(r.Context())
		if err != nil {
			writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
			go finishRequest(r, traceID, parent, spanID, start, 500, err.Error(), nil, delay)
			return
		}
		children := []span{
			mkSpan("GET/SETEX tickets cache + SELECT", 3, traceID, spanID, start, time.Now(), map[string]string{"db.system": "redis", "peer.service": "demo-valkey", "server.address": valkeyHost}, map[string]int64{"server.port": int64(atoi(valkeyPort)), "db.response.returned_rows": int64(len(items))}, 1, ""),
			mkSpan("SELECT tickets", 3, traceID, spanID, start, time.Now(), map[string]string{"db.system": "postgresql", "db.operation": "SELECT", "peer.service": "demo-postgres", "server.address": pgHost, "db.name": "shop"}, map[string]int64{"server.port": int64(atoi(pgPort)), "db.response.returned_rows": int64(len(items))}, 1, ""),
		}
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, 200, map[string]any{"items": items, "source": serviceName})
		go finishRequest(r, traceID, parent, spanID, start, 200, "", children, delay)
	case http.MethodPost:
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if payload == nil {
			payload = map[string]any{}
		}
		fail := asBool(payload["fail"]) || r.Header.Get("X-Demo-Fail") == "1"
		if fail {
			w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
			writeJSON(w, 500, map[string]any{"ok": false, "error": "ticket_create_failed"})
			go finishRequest(r, traceID, parent, spanID, start, 500, "ticket create failed", nil, delay)
			return
		}
		subject, _ := payload["subject"].(string)
		if subject == "" {
			subject = "Untitled"
		}
		email, _ := payload["email"].(string)
		if email == "" {
			email = "demo@example.com"
		}
		dbStart := time.Now()
		id, err := insertTicket(r.Context(), subject, email)
		dbEnd := time.Now()
		if err != nil {
			writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
			go finishRequest(r, traceID, parent, spanID, start, 500, err.Error(), nil, delay)
			return
		}
		_ = rdb.Del(r.Context(), "tickets:all").Err()
		children := []span{
			mkSpan("INSERT tickets", 3, traceID, spanID, dbStart, dbEnd, map[string]string{"db.system": "postgresql", "db.operation": "INSERT", "peer.service": "demo-postgres", "server.address": pgHost, "db.name": "shop"}, map[string]int64{"server.port": int64(atoi(pgPort)), "ticket.id": id}, 1, ""),
			mkSpan("DEL tickets:all", 3, traceID, spanID, dbEnd, time.Now(), map[string]string{"db.system": "redis", "db.operation": "DEL", "peer.service": "demo-valkey", "server.address": valkeyHost}, map[string]int64{"server.port": int64(atoi(valkeyPort))}, 1, ""),
		}
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, 200, map[string]any{"ok": true, "id": id, "subject": subject})
		go finishRequest(r, traceID, parent, spanID, start, 200, "", children, delay)
	default:
		writeJSON(w, 405, map[string]any{"ok": false, "error": "method_not_allowed"})
		go finishRequest(r, traceID, parent, spanID, start, 405, "method not allowed", nil, delay)
	}
}

func handlePortalStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	traceID, parent := parseTP(r.Header.Get("traceparent"))
	spanID := newID(16)
	delay := applyDemoDelay(r)
	if code, msg := demoForceError(r); code > 0 {
		w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
		writeJSON(w, code, map[string]any{"ok": false, "error": msg})
		go finishRequest(r, traceID, parent, spanID, start, code, msg, nil, delay)
		return
	}
	if appRole != "all" && appRole != "portal" {
		writeJSON(w, 404, map[string]any{"ok": false, "error": "use_demo_portal_api"})
		go finishRequest(r, traceID, parent, spanID, start, 404, "not found", nil, delay)
		return
	}
	open, pending, resolved, err := ticketCounts(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
		go finishRequest(r, traceID, parent, spanID, start, 500, err.Error(), nil, delay)
		return
	}
	n, _ := rdb.Incr(r.Context(), "portal:status:hits").Result()
	children := []span{
		mkSpan("SELECT ticket counts", 3, traceID, spanID, start, time.Now(), map[string]string{"db.system": "postgresql", "db.operation": "SELECT", "peer.service": "demo-postgres", "server.address": pgHost}, map[string]int64{"server.port": int64(atoi(pgPort))}, 1, ""),
		mkSpan("INCR portal:status:hits", 3, traceID, spanID, start, time.Now(), map[string]string{"db.system": "redis", "db.operation": "INCR", "peer.service": "demo-valkey", "server.address": valkeyHost}, map[string]int64{"server.port": int64(atoi(valkeyPort)), "portal.status.hits": n}, 1, ""),
	}
	w.Header().Set("traceresponse", "00-"+traceID+"-"+spanID+"-01")
	writeJSON(w, 200, map[string]any{
		"ok":       true,
		"service":  serviceName,
		"open":     open,
		"pending":  pending,
		"resolved": resolved,
		"hits":     n,
	})
	go finishRequest(r, traceID, parent, spanID, start, 200, "", children, delay)
}

type ticket struct {
	ID      int    `json:"id"`
	Subject string `json:"subject"`
	Email   string `json:"email"`
	Status  string `json:"status"`
}

func ensureTicketsSchema(ctx context.Context) {
	if pgPool == nil {
		return
	}
	_, err := pgPool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS tickets (
  id         SERIAL PRIMARY KEY,
  subject    TEXT NOT NULL,
  email      TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	if err != nil {
		log.Printf("tickets schema: %v", err)
		return
	}
	var n int
	_ = pgPool.QueryRow(ctx, `SELECT count(*) FROM tickets`).Scan(&n)
	if n == 0 {
		_, _ = pgPool.Exec(ctx, `
INSERT INTO tickets (subject, email, status) VALUES
  ('Order delayed', 'alice@example.com', 'open'),
  ('Wrong size shipped', 'bob@example.com', 'pending'),
  ('Refund for Alpine Jacket', 'carol@example.com', 'resolved')`)
	}
}

func fetchTickets(ctx context.Context) ([]ticket, error) {
	connectVK.Add(1)
	activeVK.Add(1)
	defer activeVK.Add(-1)
	if raw, err := rdb.Get(ctx, "tickets:all").Bytes(); err == nil {
		var items []ticket
		if json.Unmarshal(raw, &items) == nil {
			return items, nil
		}
	}
	connectPG.Add(1)
	activePG.Add(1)
	defer activePG.Add(-1)
	if pgPool == nil {
		return nil, fmt.Errorf("postgres unavailable")
	}
	rows, err := pgPool.Query(ctx, `SELECT id, subject, email, status FROM tickets ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ticket
	for rows.Next() {
		var t ticket
		if err := rows.Scan(&t.ID, &t.Subject, &t.Email, &t.Status); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if b, err := json.Marshal(items); err == nil {
		_ = rdb.Set(ctx, "tickets:all", b, cacheTTL).Err()
	}
	return items, rows.Err()
}

func insertTicket(ctx context.Context, subject, email string) (int64, error) {
	connectPG.Add(1)
	activePG.Add(1)
	defer activePG.Add(-1)
	if pgPool == nil {
		return 0, fmt.Errorf("postgres unavailable")
	}
	var id int64
	err := pgPool.QueryRow(ctx, `INSERT INTO tickets (subject, email, status) VALUES ($1,$2,'open') RETURNING id`, subject, email).Scan(&id)
	return id, err
}

func ticketCounts(ctx context.Context) (open, pending, resolved int, err error) {
	connectPG.Add(1)
	activePG.Add(1)
	defer activePG.Add(-1)
	if pgPool == nil {
		return 0, 0, 0, fmt.Errorf("postgres unavailable")
	}
	err = pgPool.QueryRow(ctx, `
SELECT
  count(*) FILTER (WHERE status = 'open'),
  count(*) FILTER (WHERE status = 'pending'),
  count(*) FILTER (WHERE status = 'resolved')
FROM tickets`).Scan(&open, &pending, &resolved)
	return
}

func fetchProducts(ctx context.Context) ([]product, error) {
	connectVK.Add(1)
	activeVK.Add(1)
	defer activeVK.Add(-1)
	if raw, err := rdb.Get(ctx, "products:all").Bytes(); err == nil {
		var items []product
		if json.Unmarshal(raw, &items) == nil {
			return items, nil
		}
	}
	connectPG.Add(1)
	activePG.Add(1)
	defer activePG.Add(-1)
	if pgPool == nil {
		return nil, fmt.Errorf("postgres unavailable")
	}
	rows, err := pgPool.Query(ctx, `SELECT id, name, price, stock FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []product
	for rows.Next() {
		var p product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if b, err := json.Marshal(items); err == nil {
		_ = rdb.Set(ctx, "products:all", b, cacheTTL).Err()
	}
	return items, rows.Err()
}

func incrCheckout(ctx context.Context) (int64, error) {
	connectVK.Add(1)
	activeVK.Add(1)
	defer activeVK.Add(-1)
	return rdb.Incr(ctx, "checkout:count").Result()
}

func insertOrder(ctx context.Context, id string, total int, email string) error {
	connectPG.Add(1)
	activePG.Add(1)
	defer activePG.Add(-1)
	if pgPool == nil {
		return fmt.Errorf("postgres unavailable")
	}
	_, err := pgPool.Exec(ctx, `INSERT INTO orders (order_id, total, currency, email) VALUES ($1,$2,'USD',$3)`, id, total, email)
	return err
}

type span struct {
	TraceID, SpanID, Parent, Name string
	Kind                          int
	Start, End                    time.Time
	Status                        int
	Msg                           string
	S                             map[string]string
	I                             map[string]int64
}

func mkSpan(name string, kind int, tid, parent string, a, b time.Time, s map[string]string, i map[string]int64, st int, msg string) span {
	return span{TraceID: tid, SpanID: newID(16), Parent: parent, Name: name, Kind: kind, Start: a, End: b, Status: st, Msg: msg, S: s, I: i}
}

func exportTrace(r *http.Request, tid, parent, sid string, start time.Time, code int, msg string, children []span) {
	st := 1
	if code >= 400 {
		st = 2
	}
	_, p, _ := net.SplitHostPort(listenAddr)
	port, _ := strconv.Atoi(p)
	server := span{
		TraceID: tid, SpanID: sid, Parent: parent, Name: r.Method + " " + r.URL.Path, Kind: 2,
		Start: start, End: time.Now(), Status: st, Msg: msg,
		S: map[string]string{"http.method": r.Method, "http.route": r.URL.Path, "http.url": fmt.Sprintf("http://localhost:%d%s", port, r.URL.Path), "server.address": "localhost"},
		I: map[string]int64{"http.status_code": int64(code), "server.port": int64(port)},
	}
	all := append([]span{server}, children...)
	type kv struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	as := func(k, v string) kv { return kv{k, map[string]string{"stringValue": v}} }
	ai := func(k string, v int64) kv { return kv{k, map[string]string{"intValue": strconv.FormatInt(v, 10)}} }
	out := make([]map[string]any, 0, len(all))
	for _, s := range all {
		attrs := []kv{}
		for k, v := range s.S {
			attrs = append(attrs, as(k, v))
		}
		for k, v := range s.I {
			attrs = append(attrs, ai(k, v))
		}
		m := map[string]any{
			"traceId": s.TraceID, "spanId": s.SpanID, "name": s.Name, "kind": s.Kind,
			"startTimeUnixNano": strconv.FormatInt(s.Start.UnixNano(), 10),
			"endTimeUnixNano":   strconv.FormatInt(s.End.UnixNano(), 10),
			"attributes":        attrs, "status": map[string]any{"code": s.Status, "message": s.Msg},
		}
		if s.Parent != "" {
			m["parentSpanId"] = s.Parent
		}
		out = append(out, m)
	}
	body, _ := json.Marshal(map[string]any{"resourceSpans": []map[string]any{{
		"resource":   map[string]any{"attributes": []kv{as("service.name", serviceName), as("service.version", serviceVersion), as("telemetry.sdk.language", "go"), as("telemetry.sdk.name", "demo-api-manual")}},
		"scopeSpans": []map[string]any{{"scope": map[string]string{"name": "demo-api", "version": serviceVersion}, "spans": out}},
	}}})
	req, err := http.NewRequest(http.MethodPost, corootOTLP, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("OTLP: %v", err)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func finishRequest(r *http.Request, tid, parent, sid string, start time.Time, code int, msg string, children []span, delayMs int) {
	durMs := time.Since(start).Milliseconds()
	exportTrace(r, tid, parent, sid, start, code, msg, children)
	sevText, sevNum := "INFO", int32(9)
	switch {
	case code >= 500:
		sevText, sevNum = "ERROR", 17
	case code >= 400:
		sevText, sevNum = "ERROR", 17
	case delayMs >= 1000:
		sevText, sevNum = "WARN", 13
	}
	body := fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, code, durMs)
	attrs := map[string]string{
		"http.method":      r.Method,
		"http.route":       r.URL.Path,
		"http.status_code": strconv.Itoa(code),
		"http.duration_ms": strconv.FormatInt(durMs, 10),
	}
	if delayMs > 0 {
		attrs["demo.delay_ms"] = strconv.Itoa(delayMs)
	}
	if msg != "" {
		attrs["error.message"] = msg
	}
	if code >= 400 {
		attrs["error.type"] = http.StatusText(code)
	}
	exportLog(sevText, sevNum, body, tid, sid, attrs)
}

func exportLog(severityText string, severityNumber int32, body, traceID, spanID string, attrs map[string]string) {
	kvAttrs := make([]*commonv1.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kvAttrs = append(kvAttrs, &commonv1.KeyValue{
			Key:   k,
			Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: v}},
		})
	}
	req := &colllogsv1.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1.ResourceLogs{{
			Resource: &resourcev1.Resource{
				Attributes: []*commonv1.KeyValue{
					{Key: "service.name", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: serviceName}}},
					{Key: "service.version", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: serviceVersion}}},
				},
			},
			ScopeLogs: []*logsv1.ScopeLogs{{
				Scope: &commonv1.InstrumentationScope{Name: "demo-api", Version: serviceVersion},
				LogRecords: []*logsv1.LogRecord{{
					TimeUnixNano:   uint64(time.Now().UnixNano()),
					SeverityNumber: logsv1.SeverityNumber(severityNumber),
					SeverityText:   severityText,
					Body:           &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: body}},
					Attributes:     kvAttrs,
					TraceId:        decodeHexID(traceID),
					SpanId:         decodeHexID(spanID),
				}},
			}},
		}},
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		log.Printf("logs marshal: %v", err)
		return
	}
	httpReq, err := http.NewRequest(http.MethodPost, corootLogsURL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("X-API-Key", apiKey)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("logs: %v", err)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("logs: status %d", resp.StatusCode)
	}
}

func applyDemoDelay(r *http.Request) int {
	ms := 0
	if v := r.URL.Query().Get("delay_ms"); v != "" {
		ms, _ = strconv.Atoi(v)
	}
	if v := r.Header.Get("X-Demo-Delay"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ms = n
		}
	}
	if ms < 0 {
		ms = 0
	}
	if ms > 15000 {
		ms = 15000
	}
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	return ms
}

func demoForceError(r *http.Request) (int, string) {
	raw := r.URL.Query().Get("force_error")
	if raw == "" {
		raw = r.Header.Get("X-Demo-Error")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, ""
	}
	code, err := strconv.Atoi(raw)
	if err != nil || code < 400 || code > 599 {
		code = 500
	}
	msg := http.StatusText(code)
	if msg == "" {
		msg = "forced_error"
	}
	return code, strings.ToLower(strings.ReplaceAll(msg, " ", "_"))
}

func decodeHexID(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

func metricsLoop() {
	for {
		resolveAddrs()
		cpuMu.Lock()
		cpuAcc += 0.03 + float64(runtime.NumGoroutine())*0.0004
		cpu := cpuAcc
		cpuMu.Unlock()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		uptime := time.Since(startAt).Seconds()
		ts := time.Now().UnixMilli()
		cores := float64(runtime.NumCPU())
		node := []lb{{"machine_id", machineID}, {"system_uuid", systemUUID}}
		common := append(append([]lb{}, node...), lb{"container_id", containerID})
		series := []tseries{
			mkTS("node_info", append(append([]lb{}, node...), lb{"hostname", hostname}, lb{"kernel_version", "6.8.0-lab"}), 1, ts),
			mkTS("node_agent_info", append(append([]lb{}, node...), lb{"version", "rum-lab-go"}), 1, ts),
			mkTS("node_uptime_seconds", node, uptime, ts),
			mkTS("node_resources_cpu_logical_cores", node, cores, ts),
			mkTS("node_resources_cpu_usage_seconds_total", append(append([]lb{}, node...), lb{"mode", "user"}), cpu, ts),
			mkTS("node_resources_cpu_usage_seconds_total", append(append([]lb{}, node...), lb{"mode", "idle"}), uptime*math.Max(1, cores-1), ts),
			mkTS("node_resources_memory_total_bytes", node, 8<<30, ts),
			mkTS("node_resources_memory_available_bytes", node, 4<<30, ts),
			mkTS("node_resources_memory_free_bytes", node, 2<<30, ts),
			mkTS("node_resources_memory_cached_bytes", node, 1<<30, ts),
			mkTS("container_info", append(append([]lb{}, common...), lb{"image", "coroot-rum-lab-demo-api:local"}), 1, ts),
			mkTS("container_resources_cpu_usage_seconds_total", common, cpu, ts),
			mkTS("container_resources_cpu_limit_cores", common, 1, ts),
			mkTS("container_resources_cpu_delay_seconds_total", common, cpu*0.05, ts),
			mkTS("container_resources_cpu_throttled_seconds_total", common, 0, ts),
			mkTS("container_resources_memory_rss_bytes", common, float64(ms.Alloc), ts),
			mkTS("container_resources_memory_cache_bytes", common, 0, ts),
			mkTS("container_resources_memory_limit_bytes", common, 512<<20, ts),
			mkTS("container_application_type", append(append([]lb{}, common...), lb{"application_type", "golang"}), 1, ts),
			mkTS("container_net_tcp_successful_connects_total", append(append([]lb{}, common...), lb{"destination", pgAddr}, lb{"actual_destination", pgAddr}), float64(connectPG.Load()), ts),
			mkTS("container_net_tcp_active_connections", append(append([]lb{}, common...), lb{"destination", pgAddr}, lb{"actual_destination", pgAddr}), math.Max(0, float64(activePG.Load())), ts),
			mkTS("container_net_tcp_successful_connects_total", append(append([]lb{}, common...), lb{"destination", valkeyAddr}, lb{"actual_destination", valkeyAddr}), float64(connectVK.Load()), ts),
			mkTS("container_net_tcp_active_connections", append(append([]lb{}, common...), lb{"destination", valkeyAddr}, lb{"actual_destination", valkeyAddr}), math.Max(0, float64(activeVK.Load())), ts),
		}
		if err := rw(metricsURL, series); err != nil {
			log.Printf("metrics: %v", err)
		}
		time.Sleep(time.Duration(metricsEvery * float64(time.Second)))
	}
}

func handleMetricsText(w http.ResponseWriter, _ *http.Request) {
	cpuMu.Lock()
	cpu := cpuAcc
	cpuMu.Unlock()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "demo_api_up{service=%q} 1\nprocess_cpu_seconds_total{service=%q} %f\nprocess_resident_memory_bytes{service=%q} %d\n", serviceName, serviceName, cpu, serviceName, ms.Alloc)
}

// Profile self-scrape → Coroot /v1/profiles (same shape as coroot-cluster-agent).
func profilesLoop() {
	time.Sleep(15 * time.Second)
	types := []string{"profile", "heap", "goroutine", "mutex", "block"}
	for {
		for _, pt := range types {
			if err := scrapeAndUpload(pt); err != nil {
				log.Printf("profile %s: %v", pt, err)
			}
		}
		time.Sleep(time.Duration(profilesEvery * float64(time.Second)))
	}
}

func scrapeAndUpload(profileType string) error {
	u := "http://127.0.0.1" + listenPortPath() + "/debug/pprof/" + profileType
	if profileType == "profile" {
		u += "?seconds=5"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	p, err := profile.Parse(resp.Body)
	if err != nil {
		return err
	}
	if len(p.Sample) == 0 {
		return nil
	}
	if p.DurationNanos == 0 {
		p.DurationNanos = int64(profilesEvery * 1e9)
	}
	rewriteProfileTypes(profileType, p)
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		return err
	}
	q := profilesURL + "?service.name=" + urlQuery(containerID)
	ureq, err := http.NewRequest(http.MethodPost, q, &buf)
	if err != nil {
		return err
	}
	ureq.Header.Set("X-API-Key", apiKey)
	uresp, err := http.DefaultClient.Do(ureq)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, uresp.Body)
	uresp.Body.Close()
	if uresp.StatusCode != 200 {
		return fmt.Errorf("upload %d", uresp.StatusCode)
	}
	return nil
}

func rewriteProfileTypes(profileType string, p *profile.Profile) {
	for i, st := range p.SampleType {
		cumulative := false
		switch profileType {
		case "profile":
			if st.Type == "samples" {
				p.SampleType[i].Type = ""
				continue
			}
		case "heap":
			if st.Type == "alloc_objects" || st.Type == "alloc_space" {
				cumulative = true
			}
		case "mutex", "block":
			cumulative = true
		}
		p.SampleType[i].Type = fmt.Sprintf("go:%s_%s:%s", profileType, st.Type, st.Unit)
		if !cumulative {
			continue
		}
		key := p.SampleType[i].Type
		profileMu.Lock()
		if profilePrev[key] == nil {
			profilePrev[key] = map[uint64]int64{}
			for _, s := range p.Sample {
				h := stackHash(s)
				profilePrev[key][h] = s.Value[i]
			}
			profileMu.Unlock()
			continue
		}
		for _, s := range p.Sample {
			h := stackHash(s)
			prev := profilePrev[key][h]
			cur := s.Value[i]
			profilePrev[key][h] = cur
			if cur-prev >= 0 {
				s.Value[i] = cur - prev
			}
		}
		profileMu.Unlock()
	}
}

func stackHash(s *profile.Sample) uint64 {
	h := fnvNew()
	for _, loc := range s.Location {
		for _, line := range loc.Line {
			if line.Function == nil {
				continue
			}
			h.Write([]byte(line.Function.Name))
			h.Write([]byte(line.Function.Filename))
			h.Write([]byte(strconv.Itoa(int(line.Line))))
		}
	}
	return h.Sum64()
}

type fnv64 struct{ sum uint64 }

func fnvNew() *fnv64 { return &fnv64{sum: 14695981039346656037} }
func (f *fnv64) Write(p []byte) {
	for _, b := range p {
		f.sum ^= uint64(b)
		f.sum *= 1099511628211
	}
}
func (f *fnv64) Sum64() uint64 { return f.sum }

type lb struct{ N, V string }
type tseries struct {
	L []lb
	S [][2]float64
}

func mkTS(name string, labels []lb, v float64, ts int64) tseries {
	return tseries{L: append([]lb{{"__name__", name}}, labels...), S: [][2]float64{{v, float64(ts)}}}
}

func rw(url string, series []tseries) error {
	var body []byte
	for _, s := range series {
		body = append(body, bf(1, encTS(s))...)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(snappy.Encode(nil, body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set("X-API-Key", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
func encTS(s tseries) []byte {
	var b []byte
	for _, l := range s.L {
		b = append(b, bf(1, append(sf(1, l.N), sf(2, l.V)...))...)
	}
	for _, sm := range s.S {
		b = append(b, bf(2, append(df(1, sm[0]), i64(2, int64(sm[1]))...))...)
	}
	return b
}
func vi(n uint64) []byte {
	var o []byte
	for {
		b := byte(n & 0x7f)
		n >>= 7
		if n != 0 {
			b |= 0x80
		}
		o = append(o, b)
		if n == 0 {
			return o
		}
	}
}
func k(f, w int) []byte         { return vi(uint64(f<<3 | w)) }
func bf(f int, d []byte) []byte { return append(append(k(f, 2), vi(uint64(len(d)))...), d...) }
func sf(f int, s string) []byte { return bf(f, []byte(s)) }
func df(f int, v float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
	return append(k(f, 1), buf...)
}
func i64(f int, v int64) []byte { return append(k(f, 0), vi(uint64(v))...) }

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		if origins[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, traceparent, X-Demo-Fail, X-Demo-Delay, X-Demo-Error, Accept")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "traceresponse")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next(w, r)
	}
}
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func busy(n int) {
	s := 0.0
	for i := 0; i < 20000+n*800; i++ {
		s += math.Sqrt(float64(i%97 + 1))
	}
	_ = s
	_ = make([]byte, 24*1024+n*32)
}
func resolveAddrs() {
	pgAddr = lookup(pgHost, pgPort)
	valkeyAddr = lookup(valkeyHost, valkeyPort)
}
func lookup(host, port string) string {
	ips, err := net.LookupIP(host)
	if err == nil {
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				return net.JoinHostPort(v4.String(), port)
			}
		}
	}
	return net.JoinHostPort(host, port)
}
func listenPortPath() string {
	_, p, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return ":4000"
	}
	return ":" + p
}
func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func getenvI(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}
func getenvF(k string, d float64) float64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return d
}
func ensureTraces(u string) string {
	u = strings.TrimRight(u, "/")
	if !strings.HasSuffix(u, "/v1/traces") {
		u += "/v1/traces"
	}
	return u
}
func ensureLogs(u string) string {
	u = strings.TrimRight(u, "/")
	if !strings.HasSuffix(u, "/v1/logs") {
		u += "/v1/logs"
	}
	return u
}
func lastSeg(s, d string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 && i+1 < len(s) {
		return s[i+1:]
	}
	return d
}
func splitSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			m[p] = true
		}
	}
	return m
}
func newID(n int) string {
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
func parseTP(tp string) (string, string) {
	p := strings.Split(tp, "-")
	if len(p) >= 3 && len(p[1]) == 32 && len(p[2]) == 16 {
		return p[1], p[2]
	}
	return newID(32), ""
}
func atoi(s string) int { n, _ := strconv.Atoi(s); return n }
func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case float64:
		return t != 0
	}
	return false
}
func asInt(v any, d int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	}
	return d
}
func urlQuery(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "/", "%2F"), ":", "%3A")
}
