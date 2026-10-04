package clickhouse

import (
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/timeseries"
)

// RumFilter holds optional dimension filters applied to RUM queries.
type RumFilter struct {
	PagePath string
	Browser  string
	Os       string
	Device   string
	Country  string
	Version  string
}

// RumSelection filters page-view samples by heatmap brush (time × duration).
type RumSelection struct {
	TsFrom  timeseries.Time
	TsTo    timeseries.Time
	DurFrom time.Duration
	DurTo   time.Duration
	Errors  bool
}

var allowedPercentiles = map[int]float64{
	50: 0.50,
	75: 0.75,
	90: 0.90,
	95: 0.95,
	99: 0.99,
}

// NormalizePercentile returns a ClickHouse quantile level; default is 0.75.
func NormalizePercentile(p int) (int, float64) {
	if q, ok := allowedPercentiles[p]; ok {
		return p, q
	}
	return 75, 0.75
}

func (f RumFilter) Empty() bool {
	return f.PagePath == "" && f.Browser == "" && f.Os == "" && f.Device == "" && f.Country == "" && f.Version == ""
}

// spanFilterSQL appends predicates for rum_spans columns / attributes.
func (f RumFilter) spanFilterSQL(prefix string) (string, []any) {
	var parts []string
	var args []any
	col := func(name string) string {
		if prefix == "" {
			return name
		}
		return prefix + "." + name
	}
	if f.PagePath != "" {
		parts = append(parts, fmt.Sprintf(`coalesce(
			nullIf(%s, ''),
			nullIf(%s['page.path'], ''),
			nullIf(%s['page.url.path'], ''),
			nullIf(%s['url.path'], '')
		) = @rf_page`, col("PagePath"), col("SpanAttributes"), col("SpanAttributes"), col("SpanAttributes")))
		args = append(args, clickhouse.Named("rf_page", f.PagePath))
	}
	if f.Browser != "" {
		parts = append(parts, col("BrowserName")+" = @rf_browser")
		args = append(args, clickhouse.Named("rf_browser", f.Browser))
	}
	if f.Os != "" {
		parts = append(parts, col("OsName")+" = @rf_os")
		args = append(args, clickhouse.Named("rf_os", f.Os))
	}
	if f.Device != "" {
		parts = append(parts, col("DeviceType")+" = @rf_device")
		args = append(args, clickhouse.Named("rf_device", f.Device))
	}
	if f.Country != "" {
		parts = append(parts, fmt.Sprintf(`coalesce(
			nullIf(%s['geo.country'], ''),
			nullIf(%s['geo.country'], ''),
			'unknown'
		) = @rf_country`, col("SpanAttributes"), col("ResourceAttributes")))
		args = append(args, clickhouse.Named("rf_country", f.Country))
	}
	if f.Version != "" {
		parts = append(parts, fmt.Sprintf(`coalesce(
			nullIf(%s['service.version'], ''),
			nullIf(%s['service.version'], ''),
			'unknown'
		) = @rf_version`, col("ResourceAttributes"), col("SpanAttributes")))
		args = append(args, clickhouse.Named("rf_version", f.Version))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}

// rollupFilterSQL appends predicates for rum_*_1m rollup columns.
func (f RumFilter) rollupFilterSQL(prefix string) (string, []any) {
	var parts []string
	var args []any
	col := func(name string) string {
		if prefix == "" {
			return name
		}
		return prefix + "." + name
	}
	if f.PagePath != "" {
		parts = append(parts, col("PagePath")+" = @rf_page")
		args = append(args, clickhouse.Named("rf_page", f.PagePath))
	}
	if f.Browser != "" {
		parts = append(parts, col("BrowserName")+" = @rf_browser")
		args = append(args, clickhouse.Named("rf_browser", f.Browser))
	}
	if f.Os != "" {
		parts = append(parts, col("OsName")+" = @rf_os")
		args = append(args, clickhouse.Named("rf_os", f.Os))
	}
	if f.Device != "" {
		parts = append(parts, col("DeviceType")+" = @rf_device")
		args = append(args, clickhouse.Named("rf_device", f.Device))
	}
	if f.Country != "" {
		parts = append(parts, col("Country")+" = @rf_country")
		args = append(args, clickhouse.Named("rf_country", f.Country))
	}
	if f.Version != "" {
		parts = append(parts, col("Version")+" = @rf_version")
		args = append(args, clickhouse.Named("rf_version", f.Version))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}

// eventFilterSQL appends predicates for rum_events columns / attributes.
func (f RumFilter) eventFilterSQL(prefix string) (string, []any) {
	var parts []string
	var args []any
	col := func(name string) string {
		if prefix == "" {
			return name
		}
		return prefix + "." + name
	}
	if f.PagePath != "" {
		parts = append(parts, col("PagePath")+" = @rf_page")
		args = append(args, clickhouse.Named("rf_page", f.PagePath))
	}
	if f.Browser != "" {
		parts = append(parts, col("BrowserName")+" = @rf_browser")
		args = append(args, clickhouse.Named("rf_browser", f.Browser))
	}
	if f.Os != "" {
		parts = append(parts, col("OsName")+" = @rf_os")
		args = append(args, clickhouse.Named("rf_os", f.Os))
	}
	if f.Device != "" {
		parts = append(parts, col("DeviceType")+" = @rf_device")
		args = append(args, clickhouse.Named("rf_device", f.Device))
	}
	if f.Country != "" {
		parts = append(parts, fmt.Sprintf(`coalesce(nullIf(%s['geo.country'], ''), 'unknown') = @rf_country`, col("Attributes")))
		args = append(args, clickhouse.Named("rf_country", f.Country))
	}
	if f.Version != "" {
		parts = append(parts, fmt.Sprintf(`coalesce(nullIf(%s['service.version'], ''), 'unknown') = @rf_version`, col("Attributes")))
		args = append(args, clickhouse.Named("rf_version", f.Version))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}

func pageViewSpanPredicate(alias string) string {
	col := "SpanName"
	if alias != "" {
		col = alias + ".SpanName"
	}
	return fmt.Sprintf(`(
		%s ILIKE '%%document%%Load%%'
		OR %s ILIKE '%%navigation%%'
		OR %s = 'routeChange'
		OR %s = 'softNavigation'
		OR %s ILIKE '%%page_view%%'
		OR %s ILIKE '%%pageview%%'
	)`, col, col, col, col, col, col)
}

func selectionSpanSQL(sel *RumSelection) (string, []any) {
	if sel == nil {
		return "", nil
	}
	var parts []string
	var args []any
	if !sel.TsFrom.IsZero() {
		parts = append(parts, "Timestamp >= @sel_from")
		args = append(args, clickhouse.DateNamed("sel_from", sel.TsFrom.ToStandard(), clickhouse.NanoSeconds))
	}
	if !sel.TsTo.IsZero() {
		parts = append(parts, "Timestamp < @sel_to")
		args = append(args, clickhouse.DateNamed("sel_to", sel.TsTo.ToStandard(), clickhouse.NanoSeconds))
	}
	if sel.Errors {
		parts = append(parts, "StatusCode = 'STATUS_CODE_ERROR'")
	} else {
		if sel.DurFrom > 0 {
			parts = append(parts, "Duration >= @sel_dur_from")
			args = append(args, clickhouse.Named("sel_dur_from", sel.DurFrom.Nanoseconds()))
		}
		if sel.DurTo > 0 {
			parts = append(parts, "Duration < @sel_dur_to")
			args = append(args, clickhouse.Named("sel_dur_to", sel.DurTo.Nanoseconds()))
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}
