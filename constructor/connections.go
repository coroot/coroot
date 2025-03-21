package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadAppToAppConnections(w *model.World, metrics map[string][]*model.MetricValues) {
	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "rr_connection") {
			continue
		}
		for _, mv := range metrics[queryName] {
			appId, err := model.NewApplicationIdFromString(mv.Labels["app"])
			if err != nil {
				klog.Warningln(err)
				continue
			}
			destId, err := model.NewApplicationIdFromString(mv.Labels["dest"])
			if err != nil {
				klog.Warningln(err)
				continue
			}
			app := w.GetOrCreateApplication(appId, false)
			conn := app.Upstreams[destId]
			if conn == nil {
				dest := w.GetOrCreateApplication(destId, false)
				conn = &model.AppToAppConnection{
					Application:       app,
					RemoteApplication: dest,
					RequestsCount:     map[model.Protocol]map[string]*timeseries.TimeSeries{},
					RequestsLatency:   map[model.Protocol]*timeseries.TimeSeries{},
				}
				app.Upstreams[destId] = conn
				dest.Downstreams[appId] = conn
			}
			switch queryName {
			case qRecordingRuleApplicationTCPSuccessful:
				conn.SuccessfulConnections = merge(conn.SuccessfulConnections, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationTCPActive:
				conn.Active = merge(conn.Active, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationTCPFailed:
				conn.FailedConnections = merge(conn.FailedConnections, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationTCPConnectionTime:
				conn.ConnectionTime = merge(conn.ConnectionTime, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationTCPBytesSent:
				conn.BytesSent = merge(conn.BytesSent, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationTCPBytesReceived:
				conn.BytesReceived = merge(conn.BytesReceived, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationTCPRetransmissions:
				conn.Retransmissions = merge(conn.Retransmissions, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationNetLatency:
				conn.Rtt = merge(conn.Rtt, mv.Values, timeseries.Any)
			case qRecordingRuleApplicationL7Latency:
				proto := model.Protocol(mv.Labels["proto"])
				conn.RequestsLatency[proto] = merge(conn.RequestsLatency[proto], mv.Values, timeseries.Any)
			case qRecordingRuleApplicationL7Requests:
				proto := model.Protocol(mv.Labels["proto"])
				switch proto {
				case model.ProtocolRabbitmq, model.ProtocolNats:
					proto += model.Protocol("-" + mv.Labels["method"])
				}
				if conn.RequestsCount[proto] == nil {
					conn.RequestsCount[proto] = map[string]*timeseries.TimeSeries{}
				}
				status := mv.Labels["status"]
				conn.RequestsCount[proto][status] = merge(conn.RequestsCount[proto][status], mv.Values, timeseries.NanSum)
			}
		}
	}
}
