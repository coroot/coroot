package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"text/template"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
	"k8s.io/klog"
)

type CheckId string

type CheckType int

const (
	CheckTypeEventBased CheckType = iota
	CheckTypeItemBased
	CheckTypeValueBased
	CheckTypeManual
)

type CheckUnit string

const (
	CheckUnitPercent          = "percent"
	CheckUnitSecond           = "second"
	CheckUnitByte             = "byte"
	CheckUnitSecondsPerSecond = "seconds/second"
)

func (u CheckUnit) FormatValue(v float32) string {
	switch u {
	case CheckUnitSecond:
		return utils.FormatDuration(timeseries.Duration(v), 1)
	case CheckUnitSecondsPerSecond:
		return utils.FormatDuration(timeseries.Duration(v), 1) + "/second"
	case CheckUnitByte:
		value, unit := utils.FormatBytes(v)
		return value + unit
	case CheckUnitPercent:
		return utils.FormatPercentage(v)
	}
	return utils.FormatFloat(v)
}

type CheckConfig struct {
	Id    CheckId
	Type  CheckType
	Title string

	DefaultThreshold        float32
	Unit                    CheckUnit
	MessageTemplate         string
	ConditionFormatTemplate string
}

var Checks = struct {
	index map[CheckId]*CheckConfig

	SLOAvailability        CheckConfig
	SLOLatency             CheckConfig
	CPUNode                CheckConfig
	CPUContainer           CheckConfig
	MemoryOOM              CheckConfig
	MemoryLeakPercent      CheckConfig
	StorageSpace           CheckConfig
	StorageIOLoad          CheckConfig
	NetworkRTT             CheckConfig
	NetworkConnectivity    CheckConfig
	NetworkTCPConnections  CheckConfig
	InstanceAvailability   CheckConfig
	DeploymentStatus       CheckConfig
	InstanceRestarts       CheckConfig
	RedisAvailability      CheckConfig
	RedisLatency           CheckConfig
	MongodbAvailability    CheckConfig
	MongodbReplicationLag  CheckConfig
	MemcachedAvailability  CheckConfig
	PostgresAvailability   CheckConfig
	PostgresLatency        CheckConfig
	PostgresReplicationLag CheckConfig
	PostgresConnections    CheckConfig
	LogErrors              CheckConfig
	JvmAvailability        CheckConfig
	JvmSafepointTime       CheckConfig
	DotNetAvailability     CheckConfig
	PythonGILWaitingTime   CheckConfig
	DnsLatency             CheckConfig
	DnsServerErrors        CheckConfig
	DnsNxdomainErrors      CheckConfig
	MysqlAvailability      CheckConfig
	MysqlReplicationStatus CheckConfig
	MysqlReplicationLag    CheckConfig
	MysqlConnections       CheckConfig
}{
	index: map[CheckId]*CheckConfig{},

	SLOAvailability: CheckConfig{
		Type:                    CheckTypeManual,
		Title:                   "Availability",
		MessageTemplate:         `the app is serving errors`,
		DefaultThreshold:        99,
		Unit:                    CheckUnitPercent,
		ConditionFormatTemplate: "the successful request percentage < <threshold>",
	},
	SLOLatency: CheckConfig{
		Type:                    CheckTypeManual,
		Title:                   "Latency",
		MessageTemplate:         `the app is performing slowly`,
		DefaultThreshold:        99,
		Unit:                    CheckUnitPercent,
		ConditionFormatTemplate: "the percentage of requests served faster than <bucket> < <threshold>",
	},
	CPUNode: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Node CPU utilization",
		MessageTemplate:         `high CPU utilization of {{.Items "node"}}`,
		DefaultThreshold:        80,
		Unit:                    CheckUnitPercent,
		ConditionFormatTemplate: "the CPU usage of a node > <threshold>",
	},
	CPUContainer: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Container CPU utilization",
		DefaultThreshold:        80,
		Unit:                    CheckUnitPercent,
		MessageTemplate:         `high CPU utilization of {{.Items "container"}}`,
		ConditionFormatTemplate: "the CPU usage of a container > <threshold> of its CPU limit",
	},
	MemoryOOM: CheckConfig{
		Type:                    CheckTypeEventBased,
		Title:                   "Out of Memory",
		DefaultThreshold:        0,
		MessageTemplate:         `app containers have been restarted {{.Count "time"}} by the OOM killer`,
		ConditionFormatTemplate: "the number of container terminations due to Out of Memory > <threshold>",
	},
	MemoryLeakPercent: CheckConfig{
		Type:                    CheckTypeValueBased,
		Title:                   "Memory leak",
		DefaultThreshold:        10,
		MessageTemplate:         `memory usage is growing by {{.Value}} %% per hour`,
		ConditionFormatTemplate: "memory usage is growing by > <threshold> % per hour",
	},
	StorageIOLoad: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Disk I/O load",
		DefaultThreshold:        5,
		Unit:                    CheckUnitSecondsPerSecond,
		MessageTemplate:         `high I/O load of {{.Items "volume"}}`,
		ConditionFormatTemplate: "the I/O load of a volume > <threshold>",
	},
	StorageSpace: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Disk space",
		DefaultThreshold:        80,
		Unit:                    CheckUnitPercent,
		MessageTemplate:         `disk space on {{.Items "volume"}} will be exhausted soon`,
		ConditionFormatTemplate: "the space usage of a volume > <threshold>",
	},
	NetworkRTT: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Network round-trip time (RTT)",
		DefaultThreshold:        0.01,
		Unit:                    CheckUnitSecond,
		MessageTemplate:         `high network latency to {{.Items "upstream service"}}`,
		ConditionFormatTemplate: "the RTT to an upstream service > <threshold>",
	},
	NetworkConnectivity: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Network connectivity",
		DefaultThreshold:        0,
		MessageTemplate:         `no connectivity with {{.Items "upstream service"}}`,
		ConditionFormatTemplate: "the number of unavailable upstream services > <threshold>",
	},
	NetworkTCPConnections: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "TCP connections",
		DefaultThreshold:        0,
		MessageTemplate:         `failed to connect to {{.Items "upstream service"}}`,
		ConditionFormatTemplate: "the number of upstream services to which the app failed to connect > <threshold>",
	},
	InstanceAvailability: CheckConfig{
		Type:                    CheckTypeManual,
		Title:                   "Instance availability",
		DefaultThreshold:        75,
		Unit:                    CheckUnitPercent,
		MessageTemplate:         `{{.ItemsWithToBe "instance"}} unavailable`,
		ConditionFormatTemplate: "the number of available instances < <threshold> of the desired",
	},
	InstanceRestarts: CheckConfig{
		Type:                    CheckTypeEventBased,
		Title:                   "Restarts",
		DefaultThreshold:        0,
		MessageTemplate:         `app containers have been restarted {{.Count "time"}}`,
		ConditionFormatTemplate: "the number of container restarts > <threshold>",
	},
	DeploymentStatus: CheckConfig{
		Type:                    CheckTypeValueBased,
		Title:                   "Deployment status",
		DefaultThreshold:        180,
		Unit:                    CheckUnitSecond,
		MessageTemplate:         `the rollout has already been in progress for {{.Value}}`,
		ConditionFormatTemplate: "a rollout is in progress > <threshold>",
	},
	RedisAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Redis availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe "redis instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable redis instances > <threshold>",
	},
	RedisLatency: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Redis latency",
		DefaultThreshold:        0.005,
		Unit:                    CheckUnitSecond,
		MessageTemplate:         `{{.ItemsWithToBe "redis instance"}} performing slowly`,
		ConditionFormatTemplate: "the average command execution time of a redis instance > <threshold>",
	},
	MongodbAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Mongodb availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe "mongodb instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable mongodb instances > <threshold>",
	},
	MongodbReplicationLag: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Mongodb replication lag",
		DefaultThreshold:        30,
		MessageTemplate:         `{{.ItemsWithToBe "mongodb replica"}} far behind the primary`,
		ConditionFormatTemplate: "replication lag > <threshold>",
		Unit:                    CheckUnitSecond,
	},
	MemcachedAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Memcached availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe "memcached instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable memcached instances > <threshold>",
	},
	PostgresAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Postgres availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe "postgres instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable postgres instances > <threshold>",
	},
	PostgresLatency: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Postgres latency",
		DefaultThreshold:        0.1,
		Unit:                    CheckUnitSecond,
		MessageTemplate:         `{{.ItemsWithToBe "postgres instance"}} performing slowly`,
		ConditionFormatTemplate: "the average query execution time of a postgres instance > <threshold>",
	},
	PostgresReplicationLag: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Postgres replication lag",
		DefaultThreshold:        30,
		MessageTemplate:         `{{.ItemsWithToBe "postgres replica"}} far behind the primary`,
		ConditionFormatTemplate: "replication lag > <threshold>",
		Unit:                    CheckUnitSecond,
	},
	PostgresConnections: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Postgres connections",
		DefaultThreshold:        90,
		MessageTemplate:         `{{.ItemsWithHave "postgres instance"}} too many connections`,
		ConditionFormatTemplate: "the number of connections > <threshold> of `max_connections`",
		Unit:                    CheckUnitPercent,
	},
	LogErrors: CheckConfig{
		Type:                    CheckTypeEventBased,
		Title:                   "Errors",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.Count "error"}} occurred`,
		ConditionFormatTemplate: "the number of messages with the ERROR and CRITICAL severity levels > <threshold>",
	},
	JvmAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "JVM availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe "JVM instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable JVM instances > <threshold>",
	},
	JvmSafepointTime: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "JVM safepoints",
		DefaultThreshold:        0.05,
		MessageTemplate:         `high safepoint time on {{.Items "JVM instance"}}`,
		ConditionFormatTemplate: "the time application have been stopped for safepoint operations > <threshold>",
		Unit:                    CheckUnitSecond,
	},
	DotNetAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   ".NET runtime availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe ".NET instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable .NET instances > <threshold>",
	},
	PythonGILWaitingTime: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Python GIL (Global Interpreter Lock) waiting time",
		DefaultThreshold:        0.05,
		MessageTemplate:         `high GIL waiting times on {{.Items "Python instance"}}`,
		ConditionFormatTemplate: "the time Python threads have been waiting for acquiring the GIL (Global Interpreter Lock) > <threshold>",
		Unit:                    CheckUnitSecond,
	},
	DnsLatency: CheckConfig{
		Type:                    CheckTypeValueBased,
		Title:                   "DNS latency",
		DefaultThreshold:        0.1,
		Unit:                    CheckUnitSecond,
		MessageTemplate:         `high latency`,
		ConditionFormatTemplate: "the 95th percentile of DNS response times > <threshold>",
	},
	DnsServerErrors: CheckConfig{
		Type:                    CheckTypeEventBased,
		Title:                   "DNS server errors",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.Count "server DNS error"}} occurred`,
		ConditionFormatTemplate: "the number of server DNS errors (excluding NXDOMAIN) > <threshold>",
	},
	DnsNxdomainErrors: CheckConfig{
		Type:                    CheckTypeEventBased,
		Title:                   "DNS NXDOMAIN errors",
		DefaultThreshold:        0,
		MessageTemplate:         `the app received an empty DNS response {{.Count "time"}}`,
		ConditionFormatTemplate: "the number of the NXDOMAIN DNS errors (for previously valid requests) > <threshold>",
	},
	MysqlAvailability: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Mysql availability",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithToBe "mysql instance"}} unavailable`,
		ConditionFormatTemplate: "the number of unavailable mysql instances > <threshold>",
	},
	MysqlReplicationStatus: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Mysql replication status",
		DefaultThreshold:        0,
		MessageTemplate:         `{{.ItemsWithHave "mysql replica"}} issues with IO or SQL replication threads`,
		ConditionFormatTemplate: "IO or SQL replication thread is not running ",
		Unit:                    CheckUnitSecond,
	},
	MysqlReplicationLag: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Mysql replication lag",
		DefaultThreshold:        30,
		MessageTemplate:         `{{.ItemsWithToBe "mysql replica"}} far behind the primary`,
		ConditionFormatTemplate: "replication lag > <threshold>",
		Unit:                    CheckUnitSecond,
	},
	MysqlConnections: CheckConfig{
		Type:                    CheckTypeItemBased,
		Title:                   "Mysql connections",
		DefaultThreshold:        90,
		MessageTemplate:         `{{.ItemsWithHave "mysql instance"}} too many connections`,
		ConditionFormatTemplate: "the number of connections > <threshold> of `max_connections`",
		Unit:                    CheckUnitPercent,
	},
}

func init() {
	cs := reflect.ValueOf(&Checks).Elem()
	for i := 0; i < cs.NumField(); i++ {
		if !cs.Type().Field(i).IsExported() {
			continue
		}
		ch := cs.Field(i).Addr().Interface().(*CheckConfig)
		ch.Id = CheckId(cs.Type().Field(i).Name)
		Checks.index[ch.Id] = ch
	}
}

type CheckContext struct {
	items *utils.StringSet
	count int64
	value float32
	unit  CheckUnit
}

func (c CheckContext) Items(singular string) string {
	return english.Plural(c.items.Len(), singular, "")
}

func (c CheckContext) ItemsWithToBe(singular string) string {
	verb := "is"
	if c.items.Len() > 1 {
		verb = "are"
	}
	return c.Items(singular) + " " + verb
}

func (c CheckContext) ItemsWithHave(singular string) string {
	verb := "has"
	if c.items.Len() > 1 {
		verb = "have"
	}
	return c.Items(singular) + " " + verb
}

func (c CheckContext) Count(singular string) string {
	return english.Plural(int(c.count), singular, "")
}

func (c CheckContext) Value() string {
	return c.unit.FormatValue(c.value)
}

type Check struct {
	Id                      CheckId   `json:"id"`
	Title                   string    `json:"title"`
	Status                  Status    `json:"status"`
	Message                 string    `json:"message"`
	Threshold               float32   `json:"threshold"`
	Unit                    CheckUnit `json:"unit"`
	ConditionFormatTemplate string    `json:"condition_format_template"`

	typ             CheckType
	messageTemplate string
	items           *utils.StringSet
	count           int64
	desired         int64
	value           float32
	values          *timeseries.TimeSeries
	fired           bool
}

func (ch *Check) Value() float32 {
	return ch.value
}

func (ch *Check) SetValue(v float32) {
	ch.value = v
}

func (ch *Check) Values() *timeseries.TimeSeries {
	return ch.values
}
func (ch *Check) SetValues(vs *timeseries.TimeSeries) {
	ch.values = vs
}

func (ch *Check) Fire() {
	ch.fired = true
}

func (ch *Check) SetStatus(status Status, format string, a ...any) {
	ch.Status = status
	ch.Message = fmt.Sprintf(format, a...)
}

func (ch *Check) AddItem(format string, a ...any) {
	if len(a) == 0 {
		ch.items.Add(format)
		return
	}
	ch.items.Add(fmt.Sprintf(format, a...))
}

func (ch *Check) Count() int64 {
	return ch.count
}

func (ch *Check) Inc(amount int64) {
	ch.count += amount
}

func (ch *Check) ResetCounter() {
	ch.count = 0
}

func (ch *Check) Desired() int64 {
	return ch.desired
}

func (ch *Check) SetDesired(desired int64) {
	ch.desired = desired
}

func (ch *Check) Items() *utils.StringSet {
	return ch.items
}

func (ch *Check) Calc() {
	switch ch.typ {
	case CheckTypeEventBased:
		if ch.count <= int64(ch.Threshold) {
			return
		}
	case CheckTypeItemBased:
		if ch.items.Len() == 0 {
			return
		}
	case CheckTypeValueBased:
		if ch.value <= ch.Threshold {
			return
		}
	case CheckTypeManual:
		if !ch.fired {
			return
		}
	default:
		return
	}
	t, err := template.New("").Parse(ch.messageTemplate)
	if err != nil {
		ch.SetStatus(UNKNOWN, "invalid template: %s", err)
		return
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, CheckContext{items: ch.items, count: ch.count, value: ch.value, unit: ch.Unit}); err != nil {
		ch.SetStatus(UNKNOWN, "failed to render message: %s", err)
		return
	}
	ch.SetStatus(WARNING, buf.String())
}

type CheckConfigSimple struct {
	Threshold float32 `json:"threshold"`
}

type CheckConfigSLOAvailability struct {
	Custom              bool    `json:"custom"`
	TotalRequestsQuery  string  `json:"total_requests_query"`
	FailedRequestsQuery string  `json:"failed_requests_query"`
	ObjectivePercentage float32 `json:"objective_percentage"`
}

func (cfg *CheckConfigSLOAvailability) Total() string {
	return fmt.Sprintf(`sum(rate(%s[$RANGE]))`, cfg.TotalRequestsQuery)
}

func (cfg *CheckConfigSLOAvailability) Failed() string {
	return fmt.Sprintf(`sum(rate(%s[$RANGE]))`, cfg.FailedRequestsQuery)
}

type CheckConfigSLOLatency struct {
	Custom              bool    `json:"custom"`
	HistogramQuery      string  `json:"histogram_query"`
	ObjectiveBucket     float32 `json:"objective_bucket"`
	ObjectivePercentage float32 `json:"objective_percentage"`
}

func (cfg *CheckConfigSLOLatency) Histogram() string {
	return fmt.Sprintf("sum by(le)(rate(%s[$RANGE]))", cfg.HistogramQuery)
}

type CheckConfigs map[ApplicationId]map[CheckId]json.RawMessage

func (cc CheckConfigs) getRaw(appId ApplicationId, checkId CheckId) json.RawMessage {
	for _, i := range []ApplicationId{appId, {}} {
		if appConfigs, ok := cc[i]; ok {
			if cfg, ok := appConfigs[checkId]; ok {
				return cfg
			}
		}
	}
	return nil
}

func (cc CheckConfigs) GetSimple(checkId CheckId, appId ApplicationId) CheckConfigSimple {
	cfg := CheckConfigSimple{Threshold: Checks.index[checkId].DefaultThreshold}
	raw := cc.getRaw(appId, checkId)
	if raw == nil {
		return cfg
	}
	v, err := unmarshal[CheckConfigSimple](raw)
	if err != nil {
		klog.Warningln("failed to unmarshal check config:", err)
		return cfg
	}
	return v
}

func (cc CheckConfigs) GetSimpleAll(checkId CheckId, appId ApplicationId) []*CheckConfigSimple {
	def := Checks.index[checkId]
	if def == nil {
		klog.Warningln("unknown check:", checkId)
		return nil
	}
	res := []*CheckConfigSimple{{Threshold: Checks.index[checkId].DefaultThreshold}}
	ids := []ApplicationId{ApplicationIdZero}
	if !appId.IsZero() {
		ids = append(ids, appId)
	}
	for _, id := range ids {
		if appConfigs, ok := cc[id]; ok {
			if raw, ok := appConfigs[checkId]; ok {
				if cfg, err := unmarshal[CheckConfigSimple](raw); err != nil {
					klog.Warningln("failed to unmarshal check config:", err)
				} else {
					res = append(res, &cfg)
					continue
				}
			}
		}
		res = append(res, nil)
	}
	return res
}

func (cc CheckConfigs) GetByCheck(id CheckId) map[ApplicationId][]any {
	res := map[ApplicationId][]any{}
	for appId, appConfigs := range cc {
		for checkId, raw := range appConfigs {
			if checkId != id {
				continue
			}
			var cfg any
			var err error
			switch id {
			case Checks.SLOAvailability.Id:
				cfg, err = unmarshal[[]CheckConfigSLOAvailability](raw)
			case Checks.SLOLatency.Id:
				cfg, err = unmarshal[[]CheckConfigSLOLatency](raw)
			default:
				cfg, err = unmarshal[CheckConfigSimple](raw)
			}
			if err != nil {
				klog.Warningln("failed to unmarshal check config:", err)
				continue
			}
			res[appId] = append(res[appId], cfg)
		}
	}
	return res
}

func (cc CheckConfigs) GetAvailability(appId ApplicationId) (CheckConfigSLOAvailability, bool) {
	defaultCfg := CheckConfigSLOAvailability{
		Custom:              false,
		ObjectivePercentage: Checks.SLOAvailability.DefaultThreshold,
	}
	appConfigs := cc[appId]
	if appConfigs == nil {
		return defaultCfg, true
	}
	raw, ok := appConfigs[Checks.SLOAvailability.Id]
	if !ok {
		return defaultCfg, true
	}
	res, err := unmarshal[[]CheckConfigSLOAvailability](raw)
	if err != nil {
		klog.Warningln("failed to unmarshal check config:", err)
		return defaultCfg, true
	}
	if len(res) == 0 {
		return defaultCfg, true
	}
	return res[0], false
}

func (cc CheckConfigs) GetLatency(appId ApplicationId, category ApplicationCategory) (CheckConfigSLOLatency, bool) {
	objectiveBucket := float32(0.5)
	if category.Auxiliary() {
		objectiveBucket = 5
	}
	defaultCfg := CheckConfigSLOLatency{
		Custom:              false,
		ObjectivePercentage: Checks.SLOLatency.DefaultThreshold,
		ObjectiveBucket:     objectiveBucket,
	}
	appConfigs := cc[appId]
	if appConfigs == nil {
		return defaultCfg, true
	}
	raw, ok := appConfigs[Checks.SLOLatency.Id]
	if !ok {
		return defaultCfg, true
	}
	res, err := unmarshal[[]CheckConfigSLOLatency](raw)
	if err != nil {
		klog.Warningln("failed to unmarshal check config:", err)
		return defaultCfg, true
	}
	if len(res) == 0 {
		return defaultCfg, true
	}
	return res[0], false
}

func unmarshal[T any](raw json.RawMessage) (T, error) {
	var cfg T
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
