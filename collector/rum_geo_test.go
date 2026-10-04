package collector

import (
	"net/http"
	"testing"

	"github.com/coroot/coroot/db"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/stretchr/testify/assert"
)

func TestClientIPFromRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{name: "nil request", req: nil, want: ""},
		{
			name: "x forwarded for chain",
			req: func() *http.Request {
				r := &http.Request{Header: http.Header{}}
				r.Header.Set("X-Forwarded-For", " 203.0.113.1 , 198.51.100.2")
				return r
			}(),
			want: "203.0.113.1",
		},
		{
			name: "x real ip",
			req: func() *http.Request {
				r := &http.Request{Header: http.Header{}}
				r.Header.Set("X-Real-IP", " 10.0.0.5 ")
				return r
			}(),
			want: "10.0.0.5",
		},
		{
			name: "remote addr with port",
			req:  &http.Request{RemoteAddr: "192.0.2.1:54321"},
			want: "192.0.2.1",
		},
		{
			name: "remote addr ipv6 with port",
			req:  &http.Request{RemoteAddr: "[2001:db8::1]:443"},
			want: "2001:db8::1",
		},
		{
			name: "remote addr without port",
			req:  &http.Request{RemoteAddr: "127.0.0.1"},
			want: "127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, clientIPFromRequest(tt.req))
		})
	}
}

func attrKeys(attrs []*common.KeyValue) []string {
	var keys []string
	for _, a := range attrs {
		keys = append(keys, a.GetKey())
	}
	return keys
}

func TestEnrichRumGeo(t *testing.T) {
	span := &tracev1.Span{Name: "page"}
	req := &v1.ExportTraceServiceRequest{
		ResourceSpans: []*tracev1.ResourceSpans{
			{
				Resource:   &resource.Resource{},
				ScopeSpans: []*tracev1.ScopeSpans{{Spans: []*tracev1.Span{span}}},
			},
		},
	}
	httpReq := &http.Request{
		Header:     http.Header{},
		RemoteAddr: "127.0.0.1:1234",
	}

	t.Run("geo disabled adds nothing", func(t *testing.T) {
		r := cloneRumGeoReq(req)
		project := &db.Project{
			Settings: db.ProjectSettings{
				Rum: &db.RumProjectSettings{GeoEnabled: false},
			},
		}
		enrichRumGeo(r, httpReq, project)
		assert.Empty(t, r.ResourceSpans[0].Resource.Attributes)
		assert.Empty(t, r.ResourceSpans[0].ScopeSpans[0].Spans[0].Attributes)
	})

	t.Run("nil rum settings adds nothing", func(t *testing.T) {
		r := cloneRumGeoReq(req)
		project := &db.Project{Settings: db.ProjectSettings{}}
		enrichRumGeo(r, httpReq, project)
		assert.Empty(t, r.ResourceSpans[0].Resource.Attributes)
	})

	t.Run("geo enabled adds hash and local country", func(t *testing.T) {
		r := cloneRumGeoReq(req)
		project := &db.Project{
			Settings: db.ProjectSettings{
				Rum: &db.RumProjectSettings{GeoEnabled: true},
			},
		}
		enrichRumGeo(r, httpReq, project)
		resKeys := attrKeys(r.ResourceSpans[0].Resource.Attributes)
		assert.Contains(t, resKeys, "geo.ip_hash")
		assert.Contains(t, resKeys, "geo.country")
	})
}

func cloneRumGeoReq(src *v1.ExportTraceServiceRequest) *v1.ExportTraceServiceRequest {
	span := &tracev1.Span{Name: "page"}
	return &v1.ExportTraceServiceRequest{
		ResourceSpans: []*tracev1.ResourceSpans{
			{
				Resource:   &resource.Resource{},
				ScopeSpans: []*tracev1.ScopeSpans{{Spans: []*tracev1.Span{span}}},
			},
		},
	}
}
