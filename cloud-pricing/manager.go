package cloud_pricing

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"io"
	"io/ioutil"
	"k8s.io/klog"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	dumpURL        = "https://coroot.github.io/cloud-pricing/data/cloud-pricing.json.gz"
	timeout        = time.Second * 30
	updateInterval = time.Hour * 24
	dumpFileName   = "cloud-pricing.json.gz"
)

type Manager struct {
	dataDir string
	lock    sync.Mutex
	model   *Model
}

func NewManager(dataDir string) (*Manager, error) {
	if err := utils.CreateDirectoryIfNotExists(dataDir); err != nil {
		return nil, err
	}
	var err error
	m := &Manager{dataDir: dataDir}
	m.model, err = m.loadFromFile(path.Join(dataDir, dumpFileName))
	if err != nil {
		klog.Warningln("failed to update cloud pricing:", err)
	}
	go func() {
		for range time.Tick(updateInterval) {
			if err := m.updateModel(); err != nil {
				klog.Warningln("failed to update cloud pricing:", err)
			}
		}
	}()
	return m, nil
}

func (m *Manager) GetNodePricePerHour(node *model.Node) float32 {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.model == nil {
		return 0
	}
	var pricing *CloudPricing
	switch strings.ToLower(node.CloudProvider.Value()) {
	case "aws":
		pricing = m.model.AWS
	case "gcp":
		pricing = m.model.GCP
	case "azure":
		pricing = m.model.Azure
	default:
		return 0
	}
	reg, ok := pricing.Compute[node.Region.Value()]
	if !ok {
		return 0
	}
	i, ok := reg[node.InstanceType.Value()]
	if !ok {
		return 0
	}
	switch strings.ToLower(node.InstanceLifeCycle.Value()) {
	case "":
		return 0
	case "spot", "preemptible":
		return i.Spot
	}
	return i.OnDemand
}

func (m *Manager) loadFromFile(p string) (*Model, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	model := &Model{
		timestamp: st.ModTime().UTC(),
	}
	if err = json.NewDecoder(r).Decode(model); err != nil {
		return nil, err
	}
	return model, nil
}

func (m *Manager) updateModel() error {
	req, err := http.NewRequest("GET", dumpURL, nil)
	if err != nil {
		return err
	}

	if t := m.getCurrentModelTimestamp(); !t.IsZero() {
		req.Header.Set("If-Modified-Since", t.Format(http.TimeFormat))
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotModified {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(resp.Status)
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp(m.dataDir, dumpFileName)
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err = io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if t, err := time.Parse(http.TimeFormat, resp.Header.Get("last-modified")); err == nil {
		_ = os.Chtimes(tmp.Name(), t, t)
	}

	model, err := m.loadFromFile(tmp.Name())
	if err != nil {
		return err
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if err = os.Rename(tmp.Name(), path.Join(m.dataDir, dumpFileName)); err != nil {
		return err
	}
	m.model = model
	return nil
}

func (m *Manager) getCurrentModelTimestamp() time.Time {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.model == nil {
		return time.Time{}
	}
	return m.model.timestamp
}
