package constructor

import (
	"net"
	"time"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"inet.af/netaddr"
)

type QueryStats struct {
	MetricsCount int               `json:"metrics_count"`
	QueryTime    float32           `json:"query_time"`
	Failed       bool              `json:"failed"`
	Cardinality  *CardinalityStats `json:"cardinality,omitempty"`
}

type DestinationStats struct {
	Unique         int `json:"unique"`
	ExternalTotal  int `json:"external_total"`
	ExternalUnique int `json:"external_unique"`
	PrivateTotal   int `json:"private_total"`
	PrivateUnique  int `json:"private_unique"`
	FqdnTotal      int `json:"fqdn_total"`
	FqdnUnique     int `json:"fqdn_unique"`
	LoopbackTotal  int `json:"loopback_total"`

	unique  *utils.StringSet
	fqdn    *utils.StringSet
	ext     *utils.StringSet
	private *utils.StringSet
}

func (ds *DestinationStats) metric(dest string) {
	ds.unique.Add(dest)
	h, _, _ := net.SplitHostPort(dest)
	if h == "" {
		return
	}
	ip, err := netaddr.ParseIP(h)
	if err != nil {
		ds.fqdn.Add(h)
		ds.FqdnTotal++
		return
	}
	if ip.IsLoopback() {
		ds.LoopbackTotal++
		return
	}
	if utils.IsIpPrivate(ip) {
		ds.private.Add(h)
		ds.PrivateTotal++
		return
	}
	ds.ext.Add(h)
	ds.ExternalTotal++
}

func (ds *DestinationStats) flush() {
	ds.Unique = ds.unique.Len()
	ds.FqdnUnique = ds.fqdn.Len()
	ds.PrivateUnique = ds.private.Len()
	ds.ExternalUnique = ds.ext.Len()
}

func NewDestinationStats() *DestinationStats {
	return &DestinationStats{
		unique:  utils.NewStringSet(),
		fqdn:    utils.NewStringSet(),
		ext:     utils.NewStringSet(),
		private: utils.NewStringSet(),
	}
}

type CardinalityStats struct {
	ContainerId       int               `json:"container_id"`
	Destination       *DestinationStats `json:"destination,omitempty"`
	ActualDestination *DestinationStats `json:"actual_destination,omitempty"`
}

func cardinalityStats(metrics []*model.MetricValues) *CardinalityStats {
	cs := &CardinalityStats{}
	cid := utils.NewStringSet()
	for _, m := range metrics {
		if m.ContainerId != "" {
			cid.Add(m.ContainerId)
		}
		if m.Destination != "" {
			if cs.Destination == nil {
				cs.Destination = NewDestinationStats()
			}
			cs.Destination.metric(m.Destination)
		}
		if m.ActualDestination != "" {
			if cs.ActualDestination == nil {
				cs.ActualDestination = NewDestinationStats()
			}
			cs.ActualDestination.metric(m.ActualDestination)
		}
	}
	cs.ContainerId = cid.Len()
	if cs.ContainerId == 0 {
		return nil
	}
	if cs.Destination != nil {
		cs.Destination.flush()
	}
	if cs.ActualDestination != nil {
		cs.ActualDestination.flush()
	}
	return cs
}

type Profile struct {
	Stages  map[string]float32    `json:"stages"`
	Queries map[string]QueryStats `json:"queries"`
}

func (p *Profile) stage(name string, f func()) {
	if p.Stages == nil {
		f()
		return
	}
	t := time.Now()
	f()
	duration := float32(time.Since(t).Seconds())
	if duration > p.Stages[name] {
		p.Stages[name] = duration
	}
}
