package prom

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

var (
	secureTransport   *http.Transport
	insecureTransport *http.Transport

	pool = &sync.Pool{New: func() interface{} {
		return bytes.NewBuffer(nil)
	}}
)

const (
	requestTimeout = 5 * time.Minute
)

func init() {
	d := net.Dialer{Timeout: 30 * time.Second}
	secureTransport = &http.Transport{
		DialContext:           d.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: requestTimeout,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}
	insecureTransport = &http.Transport{
		DialContext:           d.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: requestTimeout,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
}

type httpClientConfig struct {
	Url           string
	BasicAuth     *utils.BasicAuth
	TlsSkipVerify bool
	ExtraSelector string
	CustomHeaders []utils.Header
	Step          timeseries.Duration
	Transport     *http.Transport
}

func NewClient(promConfig *db.IntegrationPrometheus, clickhouseConfig *db.IntegrationClickhouse) (Client, error) {
	if promConfig == nil {
		return nil, errors.New("promConfig is nil")
	}
	if promConfig.UseClickHouse {
		return newClickHouse(clickhouseConfig, promConfig.RefreshInterval)
	}

	cfg := httpClientConfig{
		Url:           promConfig.Url,
		BasicAuth:     promConfig.BasicAuth,
		TlsSkipVerify: promConfig.TlsSkipVerify,
		ExtraSelector: promConfig.ExtraSelector,
		CustomHeaders: promConfig.CustomHeaders,
		Step:          promConfig.RefreshInterval,
	}
	return newHttpClient(cfg)
}

func newHttpClient(config httpClientConfig) (Client, error) {
	u, err := url.Parse(config.Url)
	if err != nil {
		return nil, err
	}
	if config.BasicAuth != nil {
		u.User = url.UserPassword(config.BasicAuth.User, config.BasicAuth.Password)
	}
	tr := config.Transport
	if tr == nil {
		tr = secureTransport
		if config.TlsSkipVerify {
			tr = insecureTransport
		}
	}
	c := &HttpClient{
		config: config,
		url:    *u,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   requestTimeout,
		},
	}
	if config.Url == "" {
		return nil, fmt.Errorf("prometheus is not configured")
	}
	return c, nil
}

type Client interface {
	Ping(ctx context.Context) error
	GetStep(from, to timeseries.Time) (timeseries.Duration, error)
	QueryRange(ctx context.Context, query string, filterLabels FilterLabelsF, from, to timeseries.Time, step timeseries.Duration) ([]*model.MetricValues, error)
	QueryRangeHandler(r *http.Request, w http.ResponseWriter)
	MetricMetadata(r *http.Request, w http.ResponseWriter)
	LabelValues(r *http.Request, w http.ResponseWriter, labelName string)
	Series(r *http.Request, w http.ResponseWriter)
	Close()
}
