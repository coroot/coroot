package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func ReadJson(r *http.Request, dest interface{}) error {
	if body, err := ioutil.ReadAll(r.Body); err != nil {
		return fmt.Errorf(`failed to read body: %w`, err)
	} else if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("failed to unmarshal body: %w", err)
	}
	return nil
}
