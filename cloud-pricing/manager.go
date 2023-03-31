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
	dumpFileName   = "cloud-pricing.json.gz"
	dumpTimeout    = time.Second * 30
	updateInterval = time.Hour * 24
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
	m.model, err = loadFromFile(path.Join(dataDir, dumpFileName))
	if err != nil {
		if os.IsNotExist(err) {
			err = m.updateModel()
		}
		if err != nil {
			klog.Warningln("failed to load cloud pricing:", err)
		}
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

func (mgr *Manager) GetNodePricePerHour(node *model.Node) float32 {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()
	if mgr.model == nil {
		return 0
	}
	var pricing *CloudPricing
	switch strings.ToLower(node.CloudProvider.Value()) {
	case "aws":
		pricing = mgr.model.AWS
	case "gcp":
		pricing = mgr.model.GCP
	case "azure":
		pricing = mgr.model.Azure
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

func (mgr *Manager) updateModel() error {
	req, err := http.NewRequest("GET", dumpURL, nil)
	if err != nil {
		return err
	}

	if t := mgr.getCurrentModelTimestamp(); !t.IsZero() {
		req.Header.Set("If-Modified-Since", t.Format(http.TimeFormat))
	}

	ctx, cancel := context.WithTimeout(context.Background(), dumpTimeout)
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
	tmp, err := os.CreateTemp(mgr.dataDir, dumpFileName)
	if err != nil {
		return err
	}
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
	m, err := loadFromFile(tmp.Name())
	if err != nil {
		return err
	}
	mgr.lock.Lock()
	defer mgr.lock.Unlock()
	if err = os.Rename(tmp.Name(), path.Join(mgr.dataDir, dumpFileName)); err != nil {
		return err
	}
	mgr.model = m
	return nil
}

func (mgr *Manager) getCurrentModelTimestamp() time.Time {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()
	if mgr.model == nil {
		return time.Time{}
	}
	return mgr.model.timestamp
}

func loadFromFile(p string) (*Model, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	m := &Model{
		timestamp: st.ModTime().UTC(),
	}
	if err = json.NewDecoder(r).Decode(m); err != nil {
		return nil, err
	}
	return m, nil
}
