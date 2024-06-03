package auditor

import (
	"math"
	"sort"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type DnsTypeStats struct {
	Requests uint64
	NxDomain uint64
	ServFail uint64
}

func (st DnsTypeStats) Nxdomain() uint64 {
	if st.Requests == st.NxDomain { // we haven't seen OK responses
		return 0
	}
	return st.NxDomain
}

type DNSStats struct {
	Domain string
	A      DnsTypeStats
	AAAA   DnsTypeStats
	Other  DnsTypeStats
}

func (a *appAuditor) dns() {
	report := a.addReport(model.AuditReportDNS)
	latencyCheck := report.CreateCheck(model.Checks.DnsLatency)
	serverErrorsCheck := report.CreateCheck(model.Checks.DnsServerErrors)
	nxdomainCheck := report.CreateCheck(model.Checks.DnsNxdomainErrors)

	table := report.
		GetOrCreateTable("Domain", "Requests", "No results (IPv4: A)", "No results (IPv6: AAAA)", "No results (other)", "ServFail").
		SetSorted()

	requestsChart := report.GetOrCreateChart(
		"DNS requests by type, per second",
		nil,
	)
	errorsChart := report.GetOrCreateChart(
		"DNS errors, per second",
		nil,
	)
	latencyChart := report.GetOrCreateChart(
		"DNS latency, seconds",
		nil,
	)
	hist := map[float32]*timeseries.Aggregate{}
	byType := map[string]*timeseries.Aggregate{}
	errors := map[string]*timeseries.Aggregate{}
	byDomain := map[string]*DNSStats{}

	seenDNSRequests := false
	for _, instance := range a.app.Instances {
		for _, container := range instance.Containers {
			for r, byStatus := range container.DNSRequests {
				for status, ts := range byStatus {
					if !seenDNSRequests && ts.Reduce(timeseries.NanSum) > 0 {
						seenDNSRequests = true
					}
					d := byDomain[r.Domain]
					if d == nil {
						d = &DNSStats{Domain: r.Domain}
						byDomain[r.Domain] = d
					}
					v := ts.Reduce(timeseries.NanSum)
					if timeseries.IsNaN(v) {
						continue
					}
					total := uint64(math.Round(float64(v) * float64(a.w.Ctx.Step)))
					var st *DnsTypeStats
					switch r.Type {
					case "TypeA":
						st = &d.A
					case "TypeAAAA":

						st = &d.AAAA
					default:
						st = &d.Other
					}
					st.Requests += total
					switch status {
					case "ok":
					case "nxdomain":
						st.NxDomain += total
					default:
						serverErrorsCheck.Inc(int64(total))
						st.ServFail += total
					}
					if requestsChart != nil {
						t := byType[r.Type]
						if t == nil {
							t = timeseries.NewAggregate(timeseries.NanSum)
							byType[r.Type] = t
						}
						t.Add(ts)

						if status != "ok" {
							label := r.Type + ":" + status
							e := errors[label]
							if e == nil {
								e = timeseries.NewAggregate(timeseries.NanSum)
								errors[label] = e
							}
							e.Add(ts)
						}
					}
				}
			}
			for b, ts := range container.DNSRequestsHistogram {
				v := hist[b]
				if v == nil {
					v = timeseries.NewAggregate(timeseries.NanSum)
					hist[b] = v
				}
				v.Add(ts)
			}
		}
	}

	buckets := make([]model.HistogramBucket, 0, len(hist))
	for le, ts := range hist {
		buckets = append(buckets, model.HistogramBucket{Le: le, TimeSeries: ts.Get()})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Le < buckets[j].Le
	})

	domains := make([]*DNSStats, 0, len(byDomain))
	for _, d := range byDomain {
		domains = append(domains, d)
	}
	sort.Slice(domains, func(i, j int) bool {
		di := domains[i]
		dj := domains[j]
		return (di.A.Requests + di.AAAA.Requests + di.Other.Requests) > (dj.A.Requests + dj.AAAA.Requests + dj.Other.Requests)
	})
	for i, d := range domains {
		req := d.A.Requests + d.AAAA.Requests + d.Other.Requests
		if req < 1 {
			continue
		}
		status := model.OK
		if nxdomainErrors := d.A.Nxdomain() + d.AAAA.Nxdomain() + d.Other.Nxdomain(); nxdomainErrors > 0 {
			status = model.WARNING
			nxdomainCheck.Inc(int64(nxdomainErrors))
		}
		if d.A.ServFail > 0 || d.AAAA.ServFail > 0 || d.Other.ServFail > 0 {
			status = model.WARNING
		}
		if status == model.OK && (d.A.NxDomain+d.AAAA.NxDomain+d.Other.NxDomain) == req {
			status = model.UNKNOWN
		}

		if table != nil {
			if (i + 1) <= 10 {
				table.Rows = append(table.Rows, &model.TableRow{Cells: []*model.TableCell{
					model.NewTableCell().SetStatus(status, d.Domain),
					model.NewTableCell().SetEventsCount(req),
					model.NewTableCell().SetEventsCount(d.A.NxDomain),
					model.NewTableCell().SetEventsCount(d.AAAA.NxDomain),
					model.NewTableCell().SetEventsCount(d.Other.NxDomain),
					model.NewTableCell().SetEventsCount(d.A.ServFail + d.AAAA.ServFail + d.Other.ServFail),
				}})
			}
		}
	}
	if requestsChart != nil {
		requestsChart.Stacked()
		for typ, ts := range byType {
			requestsChart.AddSeries(typ, ts)
		}
	}
	if errorsChart != nil {
		errorsChart.Stacked()
		for e, ts := range errors {
			errorsChart.AddSeries(e, ts)
		}
	}
	if latencyChart != nil {
		latencyChart.PercentilesFrom(buckets, 0.5, 0.95, 0.99)
	}

	if len(buckets) > 0 {
		q95 := model.Quantile(buckets, 0.95)
		if l := q95.Last(); !timeseries.IsNaN(l) {
			latencyCheck.SetValue(l)
		}
	}

	if !seenDNSRequests {
		a.delReport(model.AuditReportDNS)
	}
}
