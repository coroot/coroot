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

var Checks struct {
	CPU struct {
		Node      CheckId `title:"Node CPU utilization"`
		Container CheckId `title:"Container CPU utilization"`
	}
	Memory struct {
		OOM CheckId `title:"OOM"`
	}
	Storage struct {
		Space CheckId `title:"Storage space usage"`
		IO    CheckId `title:"Storage IO usage"`
	}
	Network struct {
		Latency CheckId `title:"Network latency"`
	}
	Postgres struct {
		Status  CheckId `title:"Postgres status"`
		Latency CheckId `title:"Postgres latency"`
		Errors  CheckId `title:"Postgres errors"`
	}
	Redis struct {
		Status  CheckId `title:"Redis status"`
		Latency CheckId `title:"Redis latency"`
	}
	Logs struct {
		Errors CheckId `title:"Log errors"`
	}
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
	value float64
}

func (c CheckContext) Plural(singular string) string {
	return english.Plural(c.items.Len(), singular, "")
}

func (c CheckContext) IsOrAre() string {
	if c.items.Len() == 1 {
		return "is"
	}
	return "are"
}

func (c CheckContext) Value() string {
	return fmt.Sprintf("%.0f", c.value)
}

type Check struct {
	Id      CheckId `json:"id"`
	Title   string  `json:"title"`
	Status  Status  `json:"status"`
	Message string  `json:"message"`
	items   *utils.StringSet
	value   float64
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

func (ch *Check) Inc(amount float64) {
	ch.value += amount
}

func (ch *Check) Format(tmpl string, threshold ...float64) {
	switch {
	case len(threshold) > 0:
		if ch.value <= threshold[0] {
			return
		}
	default:
		if ch.items.Len() == 0 {
			return
		}
	}
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		ch.SetStatus(UNKNOWN, "invalid template: %s", err)
		return
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, CheckContext{items: ch.items, value: ch.value}); err != nil {
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
