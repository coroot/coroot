package profiling

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/pyroscope-io/pyroscope/pkg/model/appmetadata"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

const (
	metadataTimeout = 10 * time.Second
	profileTimeout  = 30 * time.Second
)

var (
	secureTransport   *http.Transport
	insecureTransport *http.Transport
)

func init() {
	d := net.Dialer{Timeout: 30 * time.Second}
	secureTransport = &http.Transport{
		DialContext:         d.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	insecureTransport = &http.Transport{
		DialContext:         d.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}
}

type PyroscopeClientConfig struct {
	Url           string
	TlsSkipVerify bool
	ApiKey        string
	BasicAuth     *utils.BasicAuth
	CustomHeaders []utils.Header
	Transport     *http.Transport
}

func NewPyroscopeClientConfig(url string) PyroscopeClientConfig {
	return PyroscopeClientConfig{Url: url}
}

type PyroscopeClient struct {
	config     PyroscopeClientConfig
	url        url.URL
	httpClient *http.Client
}

func NewPyroscopeClient(config PyroscopeClientConfig) (*PyroscopeClient, error) {
	u, err := url.Parse(config.Url)
	if err != nil {
		return nil, err
	}
	if config.BasicAuth != nil {
		u.User = url.UserPassword(config.BasicAuth.User, config.BasicAuth.Password)
	}
	if err != nil {
		return nil, err
	}
	tr := config.Transport
	if tr == nil {
		tr = secureTransport
		if config.TlsSkipVerify {
			tr = insecureTransport
		}
	}
	c := &PyroscopeClient{
		config:     config,
		url:        *u,
		httpClient: &http.Client{Transport: tr},
	}
	return c, nil
}

func (c *PyroscopeClient) Metadata(ctx context.Context) (Metadata, error) {
	var res []appmetadata.ApplicationMetadata
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	err := c.get(ctx, "/api/apps", nil, &res)
	return res, err
}

func (c *PyroscopeClient) Profile(ctx context.Context, view View, query string, from, to timeseries.Time) (*Profile, error) {
	switch view {
	case "", ViewSingle:
		return c.Single(ctx, query, from, to)
	case ViewDiff:
		return c.Diff(ctx, query, from, to)
	}
	return nil, fmt.Errorf("unknown view: %s", view)
}

func (c *PyroscopeClient) Single(ctx context.Context, query string, from, to timeseries.Time) (*Profile, error) {
	args := map[string]string{
		"from":   strconv.FormatInt(int64(from), 10),
		"until":  strconv.FormatInt(int64(to), 10),
		"query":  query,
		"format": "json",
	}
	var p Profile
	ctx, cancel := context.WithTimeout(ctx, profileTimeout)
	defer cancel()
	if err := c.get(ctx, "/render", args, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *PyroscopeClient) Diff(ctx context.Context, query string, from, to timeseries.Time) (*Profile, error) {
	args := map[string]string{
		"leftQuery":  query,
		"leftFrom":   strconv.FormatInt(int64(from.Add(-to.Sub(from))), 10),
		"leftUntil":  strconv.FormatInt(int64(from), 10),
		"rightQuery": query,
		"rightFrom":  strconv.FormatInt(int64(from), 10),
		"rightUntil": strconv.FormatInt(int64(to), 10),
		"format":     "json",
	}
	var p Profile
	ctx, cancel := context.WithTimeout(ctx, profileTimeout)
	defer cancel()
	if err := c.get(ctx, "/render-diff", args, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *PyroscopeClient) get(ctx context.Context, uri string, args map[string]string, res any) error {
	u := c.url
	u.Path = path.Join(u.Path, uri)
	q := u.Query()
	for k, v := range args {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}
	if c.config.ApiKey != "" {
		req.Header.Add("Authorization", "Bearer "+c.config.ApiKey)
	}
	for _, h := range c.config.CustomHeaders {
		req.Header.Add(h.Key, h.Value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, res)
}
