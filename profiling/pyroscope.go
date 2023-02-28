package profiling

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"github.com/pyroscope-io/pyroscope/pkg/model/appmetadata"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

const (
	metadataTimeout = 5 * time.Second
	profileTimeout  = 30 * time.Second
)

type Pyroscope struct {
	url    string
	client *http.Client
}

func NewPyroscope(url string) *Pyroscope {
	c := &Pyroscope{
		url:    url,
		client: http.DefaultClient,
	}
	return c
}

func (c *Pyroscope) Metadata(ctx context.Context) (Metadata, error) {
	var res []appmetadata.ApplicationMetadata
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	err := c.getJson(ctx, "/api/apps", nil, &res)
	return res, err
}

func (c *Pyroscope) Profile(ctx context.Context, view View, query string, from, to timeseries.Time) (*Profile, error) {
	switch view {
	case "", ViewSingle:
		return c.Single(ctx, query, from, to)
	case ViewDiff:
		return c.Diff(ctx, query, from, to)
	}
	return nil, fmt.Errorf("unknown view: %s", view)
}

func (c *Pyroscope) Single(ctx context.Context, query string, from, to timeseries.Time) (*Profile, error) {
	args := map[string]string{
		"from":   strconv.FormatInt(int64(from), 10),
		"until":  strconv.FormatInt(int64(to), 10),
		"query":  query,
		"format": "json",
	}
	var p Profile
	ctx, cancel := context.WithTimeout(ctx, profileTimeout)
	defer cancel()
	if err := c.getJson(ctx, "/render", args, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Pyroscope) Diff(ctx context.Context, query string, from, to timeseries.Time) (*Profile, error) {
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
	if err := c.getJson(ctx, "/render-diff", args, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Pyroscope) get(ctx context.Context, uri string, args map[string]string) ([]byte, error) {
	u, err := url.Parse(c.url)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, uri)
	q := u.Query()
	for k, v := range args {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	r, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Pyroscope) getJson(ctx context.Context, uri string, args map[string]string, res any) error {
	data, err := c.get(ctx, uri, args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, res)
}
