package api

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/collector"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/utils"
	"golang.org/x/net/websocket"
	"k8s.io/klog"
)

func (api *Api) ClickhouseConfig(w http.ResponseWriter, r *http.Request, project *db.Project) {
	cfg := project.ClickHouseConfig(api.globalClickHouse)
	utils.WriteJson(w, cfg)
}

func (api *Api) ClickhouseConnect(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get(collector.ApiKeyHeader)
	if apiKey == "" {
		klog.Warningln("no api key")
		http.Error(w, "no api key", http.StatusBadRequest)
		return
	}
	project, err := api.getProjectByApiKey(apiKey)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		klog.Warningln("no project found")
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	cfg := project.ClickHouseConfig(api.globalClickHouse)
	if cfg == nil {
		http.Error(w, "clickhouse is not configured", http.StatusNotFound)
		return
	}
	var upstreamConn net.Conn
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if cfg.TlsEnable {
		upstreamConn, err = tls.DialWithDialer(dialer, "tcp", cfg.Addr, ch.TlsConfig(cfg.TlsCAFile, cfg.TlsSkipVerify))
	} else {
		upstreamConn, err = dialer.Dial("tcp", cfg.Addr)
	}
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer upstreamConn.Close()

	websocket.Server{Handler: func(ws *websocket.Conn) {
		defer ws.Close()
		ws.PayloadType = websocket.BinaryFrame
		errCh := make(chan error, 2)
		go func() {
			_, e := io.Copy(upstreamConn, ws)
			errCh <- e
		}()
		go func() {
			_, e := io.Copy(ws, upstreamConn)
			errCh <- e
		}()
		<-errCh
	}}.ServeHTTP(w, r)
}
