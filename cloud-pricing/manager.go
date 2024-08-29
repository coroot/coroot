package cloud_pricing

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
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
		if err := m.updateModel(); err != nil {
			klog.Warningln("failed to update cloud pricing:", err)
		}
		for range time.Tick(updateInterval) {
			if err := m.updateModel(); err != nil {
				klog.Warningln("failed to update cloud pricing:", err)
			}
		}
	}()
	return m, nil
}

func (mgr *Manager) GetNodePrice(node *model.Node) *model.NodePrice {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()
	if mgr.model == nil {
		return nil
	}
	var pricing *CloudPricing
	var price float32
	switch strings.ToLower(node.CloudProvider.Value()) {
	case "aws":
		pricing = mgr.model.AWS
	case "gcp":
		pricing = mgr.model.GCP
	case "azure":
		pricing = mgr.model.Azure
	default:
		return nil
	}
	region := Region(strings.ToLower(node.Region.Value()))
	switch {
	case len(node.Instances) == 1 && node.Instances[0].Rds != nil: //RDS
		rds := node.Instances[0].Rds
		reg, ok := pricing.ManagedDB[region]
		if !ok {
			return nil
		}
		e, ok := reg[Engine(rds.Engine.Value())]
		if !ok {
			return nil
		}
		i, ok := e[InstanceType(node.InstanceType.Value())]
		if !ok {
			return nil
		}
		switch rds.MultiAz.Value() {
		case "", "false":
			price = i.SingleAz.OnDemand
		case "true":
			price = i.MultiAz.OnDemand
		}
	case len(node.Instances) == 1 && node.Instances[0].Elasticache != nil: //Elasticache
		ec := node.Instances[0].Elasticache
		reg, ok := pricing.ManagedCache[region]
		if !ok {
			return nil
		}
		e, ok := reg[Engine(ec.Engine.Value())]
		if !ok {
			return nil
		}
		i, ok := e[InstanceType(node.InstanceType.Value())]
		if !ok {
			return nil
		}
		price = i.OnDemand
	default: //compute
		reg, ok := pricing.Compute[region]
		if !ok {
			return nil
		}
		i, ok := reg[InstanceType(node.InstanceType.Value())]
		if !ok {
			return nil
		}
		switch strings.ToLower(node.InstanceLifeCycle.Value()) {
		case "spot", "preemptible":
			price = i.Spot
		default:
			price = i.OnDemand
		}
	}
	if !(price > 0) {
		return nil
	}
	price /= float32(timeseries.Hour)
	cpuCores := node.CpuCapacity.Last()
	memBytes := node.MemoryTotalBytes.Last()
	np := &model.NodePrice{Total: price}
	if timeseries.IsNaN(cpuCores) || timeseries.IsNaN(memBytes) {
		return np
	}
	const gb = 1e9
	perUnit := price / (cpuCores + memBytes/gb) // assume that 1Gb of memory costs the same as 1 vCPU
	np.PerCPUCore = perUnit
	np.PerMemoryByte = perUnit / gb
	return np
}

func (mgr *Manager) GetDataTransferPrice(node *model.Node) *model.DataTransferPrice {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()
	if mgr.model == nil {
		return nil
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
		return nil
	}
	if pricing.IntraRegionDataTransfer == nil || pricing.InternetEgress == nil {
		return nil
	}
	region := Region(strings.ToLower(node.Region.Value()))
	res := &model.DataTransferPrice{
		InternetPerGB: map[model.InternetStartUsageAmountGB]float32{},
	}
	p, ok := pricing.IntraRegionDataTransfer[region]
	if !ok {
		return nil
	}
	res.InterZoneIngressPerGB = float32(p.IngressPerGB)
	res.InterZoneEgressPerGB = float32(p.EgressPerGB)
	internetP, ok := pricing.InternetEgress[region]
	if !ok {
		return nil
	}
	for start, price := range internetP {
		res.InternetPerGB[model.InternetStartUsageAmountGB(start)] = float32(price)
	}
	return res
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
