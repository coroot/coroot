package utils

import (
	"math"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"k8s.io/klog"
)

func SetMemoryLimit() {
	limit, err := memlimit.Set()
	if err != nil {
		klog.Warningln("failed to set memory limit:", err)
		return
	}
	if limit == math.MaxInt64 {
		klog.Infoln("memory limit: not set")
		return
	}
	klog.Infof("memory limit: %d MiB", limit>>20)
}
