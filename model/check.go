package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
	"k8s.io/klog"
	"reflect"
	"text/template"
)

type CheckId string

type CheckType int

const (
	CheckTypeEventBased CheckType = iota
	CheckTypeItemBased
)

type CheckUnit string

const (
	CheckUnitPercent  = "percent"
	CheckUnitDuration = "duration"
)

type CheckConfig struct {
	Id    CheckId
	Type  CheckType
	Title string

	DefaultThreshold   float64
	Unit               CheckUnit
	MessageTemplate    string
	RuleFormatTemplate string
}

var Checks = struct {
	CPUNode              CheckConfig
	CPUContainer         CheckConfig
	MemoryOOM            CheckConfig
	StorageSpace         CheckConfig
	StorageIO            CheckConfig
	NetworkRTT           CheckConfig
	InstanceAvailability CheckConfig
	InstanceRestarts     CheckConfig
	RedisAvailability    CheckConfig
	RedisLatency         CheckConfig
	PostgresAvailability CheckConfig
	PostgresLatency      CheckConfig
	PostgresErrors       CheckConfig
	LogErrors            CheckConfig
}{
	CPUNode: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Node CPU utilization",
		MessageTemplate:    `high CPU utilization of {{.Items "node"}}`,
		DefaultThreshold:   80,
		Unit:               CheckUnitPercent,
		RuleFormatTemplate: "",
	},
	CPUContainer: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Container CPU utilization",
		DefaultThreshold:   80,
		Unit:               CheckUnitPercent,
		MessageTemplate:    `high CPU utilization of {{.Items "container"}}`,
		RuleFormatTemplate: "",
	},
	MemoryOOM: CheckConfig{
		Type:               CheckTypeEventBased,
		Title:              "Out of Memory events",
		DefaultThreshold:   0,
		MessageTemplate:    `app containers have been restarted {{.Count "time"}} by the OOM killer`,
		RuleFormatTemplate: "",
	},
	StorageIO: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Disk I/O",
		DefaultThreshold:   80,
		Unit:               CheckUnitPercent,
		MessageTemplate:    `high I/O utilization of {{.Items "volume"}}`,
		RuleFormatTemplate: "",
	},
	StorageSpace: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Disk space",
		DefaultThreshold:   80,
		Unit:               CheckUnitPercent,
		MessageTemplate:    `disk space on {{.Items "volume"}} will be exhausted soon`,
		RuleFormatTemplate: "",
	},
	NetworkRTT: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Network latency",
		DefaultThreshold:   0.01,
		Unit:               CheckUnitDuration,
		MessageTemplate:    `high network latency to {{.Items "upstream service"}}`,
		RuleFormatTemplate: "",
	},
	InstanceAvailability: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Instance availability",
		DefaultThreshold:   0,
		MessageTemplate:    `{{.ItemsWithToBe "instance"}} unavailable`,
		RuleFormatTemplate: "",
	},
	InstanceRestarts: CheckConfig{
		Type:               CheckTypeEventBased,
		Title:              "Restarts",
		DefaultThreshold:   0,
		MessageTemplate:    `app containers have been restarted {{.Count "time"}}`,
		RuleFormatTemplate: "",
	},
	RedisAvailability: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Redis availability",
		DefaultThreshold:   0,
		MessageTemplate:    `{{.ItemsWithToBe "redis instance"}} unavailable`,
		RuleFormatTemplate: "",
	},
	RedisLatency: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Redis latency",
		DefaultThreshold:   0.005,
		Unit:               CheckUnitDuration,
		MessageTemplate:    `{{.ItemsWithToBe "redis instance"}} performing slowly`,
		RuleFormatTemplate: "",
	},
	PostgresAvailability: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Postgres availability",
		DefaultThreshold:   0,
		MessageTemplate:    `{{.ItemsWithToBe "postgres instance"}} unavailable`,
		RuleFormatTemplate: "",
	},
	PostgresLatency: CheckConfig{
		Type:               CheckTypeItemBased,
		Title:              "Postgres latency",
		DefaultThreshold:   0.1,
		Unit:               CheckUnitDuration,
		MessageTemplate:    `{{.ItemsWithToBe "postgres instance"}} performing slowly`,
		RuleFormatTemplate: "",
	},
	PostgresErrors: CheckConfig{
		Type:               CheckTypeEventBased,
		Title:              "Postgres errors",
		DefaultThreshold:   0,
		MessageTemplate:    `{{.Count "error"}} occurred`,
		RuleFormatTemplate: "",
	},
	LogErrors: CheckConfig{
		Type:               CheckTypeEventBased,
		Title:              "Errors",
		DefaultThreshold:   0,
		MessageTemplate:    `{{.Count "error"}} occurred`,
		RuleFormatTemplate: "",
	},
}

var checkTitles = map[CheckId]string{}

func (ci CheckId) Title() string {
	return checkTitles[ci]
}

func init() {
	cs := reflect.ValueOf(&Checks).Elem()
	for i := 0; i < cs.NumField(); i++ {
		c := cs.Field(i)
		for j := 0; j < c.NumField(); j++ {
			id := cs.Type().Field(i).Name + "." + c.Type().Field(j).Name
			c.Field(j).SetString(id)
			title, _ := c.Type().Field(j).Tag.Lookup("title")
			if title == "" {
				panic("empty title for " + id)
			}
			checkTitles[CheckId(id)] = title
		}
	}
}

type CheckContext struct {
	items *utils.StringSet
	count int64
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

func (c CheckContext) Count(singular string) string {
	return english.Plural(int(c.count), singular, "")
}

type Check struct {
	Id                 CheckId   `json:"id"`
	Title              string    `json:"title"`
	Status             Status    `json:"status"`
	Message            string    `json:"message"`
	Threshold          float64   `json:"threshold"`
	Unit               CheckUnit `json:"unit"`
	RuleFormatTemplate string    `json:"rule_format_template"`

	typ             CheckType
	messageTemplate string
	items           *utils.StringSet
	count           int64
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

func (ch *Check) Inc(amount int64) {
	ch.count += amount
}

func (ch *Check) Calc() {
	switch ch.typ {
	case CheckTypeEventBased:
		if ch.count <= int64(ch.Threshold) {
			return
		}
	case CheckTypeItemBased:
		if ch.items.Len() <= int(ch.Threshold) {
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
	if err := t.Execute(buf, CheckContext{items: ch.items, count: ch.count}); err != nil {
		ch.SetStatus(UNKNOWN, "failed to render message: %s", err)
		return
	}
	ch.SetStatus(WARNING, buf.String())
}

type CheckConfigSimple struct {
	Threshold float64 `json:"threshold"`
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

func (cc CheckConfigs) GetSimple(appId ApplicationId, checkId CheckId, defaultThreshold float64) CheckConfigSimple {
	cfg := CheckConfigSimple{Threshold: defaultThreshold}
	raw := cc.getRaw(appId, checkId)
	if raw == nil {
		return cfg
	}
	var v CheckConfigSimple
	if err := json.Unmarshal(raw, &v); err != nil {
		klog.Warningln("failed to unmarshal check config:", err)
		return cfg
	}
	return v
}

//type CheckConfig struct {
//	Threshold    float64 `json:"threshold"`
//	Availability *struct {
//		TotalRequestsQuery  string `json:"total_requests_query"`
//		FailedRequestsQuery string `json:"failed_requests_query"`
//	} `json:"availability;omitempty"`
//	Latency *struct {
//		LatencyHistogramQuery string `json:"latency_histogram_query"`
//		Bucket                string `json:"bucket"`
//	} `json:"latency;omitempty"`
//}
