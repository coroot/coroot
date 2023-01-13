package pyroscope

import (
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/pyroscope-io/pyroscope/pkg/model/appmetadata"
	"github.com/pyroscope-io/pyroscope/pkg/structs/flamebearer"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Profile struct {
	Name        string                            `json:"name"`
	Flamebearer flamebearer.FlamebearerV1         `json:"flamebearer"`
	Metadata    flamebearer.FlamebearerMetadataV1 `json:"metadata"`
}

type Profiler struct {
	Meta       appmetadata.ApplicationMetadata
	Namespaces *utils.StringSet
	Pods       *utils.StringSet
}

func (p *Profiler) Match(typ string, namespace string, pods []string) bool {
	switch typ {
	case "cpu":
		if !strings.HasSuffix(p.Meta.FQName, ".cpu") {
			return false
		}
	}
	if !p.Namespaces.Has(namespace) {
		return false
	}
	for _, pod := range pods {
		if p.Pods.Has(pod) {
			return true
		}
	}
	return false
}

type Client struct {
	url    string
	client *http.Client

	profilers []*Profiler
	from, to  timeseries.Time
}

func (c *Client) get(uri string, args map[string]string) ([]byte, error) {
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
	r, err := http.NewRequest("GET", u.String(), nil)
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

func (c *Client) getJson(uri string, args map[string]string, res any) error {
	data, err := c.get(uri, args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, res)
}

func (c *Client) loadProfilers() error {
	var mds []appmetadata.ApplicationMetadata
	if err := c.getJson("/api/apps", nil, &mds); err != nil {
		return err
	}
	for _, md := range mds {
		profiler := &Profiler{Meta: md}
		var labels []string
		args := map[string]string{
			"from":  strconv.FormatInt(int64(c.from), 10),
			"until": strconv.FormatInt(int64(c.to), 10),
			"query": md.FQName,
		}
		if err := c.getJson("/labels", args, &labels); err != nil {
			return err
		}

		if ss := utils.NewStringSet(labels...); !ss.Has("pod") || !ss.Has("namespace") {
			continue
		}
		for _, l := range []string{"namespace", "pod"} {
			var values []string
			args["label"] = l
			if err := c.getJson("/label-values", args, &values); err != nil {
				return err
			}
			switch l {
			case "namespace":
				profiler.Namespaces = utils.NewStringSet(values...)
			case "pod":
				profiler.Pods = utils.NewStringSet(values...)
			}
		}
		c.profilers = append(c.profilers, profiler)
	}
	return nil
}

func (c *Client) Profiles(typ string, namespace string, pods []string) ([]*Profile, error) {
	if len(pods) == 0 {
		return nil, nil
	}
	var res []*Profile
	for _, p := range c.profilers {
		if p.Match(typ, namespace, pods) {
			q := fmt.Sprintf(`%s{namespace="%s", pod=~"(%s)"}`,
				p.Meta.FQName,
				namespace,
				strings.Join(pods, "|"),
			)
			profile, err := c.profile(p.Meta.FQName, q)
			if err != nil {
				return nil, err
			}
			res = append(res, profile)
		}
	}
	return res, nil
}

func (c *Client) profile(name, query string) (*Profile, error) {
	args := map[string]string{
		"from":   strconv.FormatInt(int64(c.from), 10),
		"until":  strconv.FormatInt(int64(c.to), 10),
		"query":  query,
		"format": "json",
	}
	profile := &Profile{Name: name}
	if err := c.getJson("/render", args, &profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func New(url string, from, to timeseries.Time) (*Client, error) {
	c := &Client{
		url:    url,
		client: http.DefaultClient,
		from:   from,
		to:     to,
	}
	if err := c.loadProfilers(); err != nil {
		return nil, err
	}
	return c, nil
}
