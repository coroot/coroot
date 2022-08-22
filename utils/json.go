package utils

import (
	"encoding/json"
	"k8s.io/klog"
	"net/http"
)

func WriteJson(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		klog.Errorf("failed to encode: %s", err)
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}
