package api

import (
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coroot/coroot/collector"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

func (api *Api) ClickhouseConfig(w http.ResponseWriter, r *http.Request, project *db.Project) {
	cfg := project.ClickHouseConfig(api.globalClickHouse)
	utils.WriteJson(w, cfg)
}

func (api *Api) ClickhouseConnect(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get(collector.ApiKeyHeader)
	hj, ok := w.(http.Hijacker)
	if !ok {
		klog.Errorln("connection hijacking not supported")
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	if apiKey == "" {
		_ = binary.Write(clientConn, binary.LittleEndian, uint32(http.StatusBadRequest))
		klog.Warningln("no api key")
		return
	}
	project, err := api.getProjectByApiKey(apiKey)
	if err != nil {
		_ = binary.Write(clientConn, binary.LittleEndian, uint32(http.StatusInternalServerError))
		klog.Errorln(err)
		return
	}
	if project == nil {
		klog.Warningln("no project found")
		_ = binary.Write(clientConn, binary.LittleEndian, uint32(http.StatusNotFound))
		return
	}
	cfg := project.ClickHouseConfig(api.globalClickHouse)

	upstreamConn, err := net.DialTimeout("tcp", cfg.Addr, 10*time.Second)
	if err != nil {
		klog.Errorln(err)
		_ = binary.Write(clientConn, binary.LittleEndian, uint32(http.StatusBadGateway))
		return
	}
	defer upstreamConn.Close()

	_ = binary.Write(clientConn, binary.LittleEndian, uint32(http.StatusOK))
	errCh := make(chan error, 2)

	go func() {
		_, e := io.Copy(upstreamConn, clientConn)
		errCh <- e
	}()
	go func() {
		_, e := io.Copy(clientConn, upstreamConn)
		errCh <- e
	}()

	<-errCh
}
