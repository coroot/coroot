package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"k8s.io/klog"
)

func WriteJson(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		klog.Errorf("failed to encode: %s", err)
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

func ReadJson(r *http.Request, dest any) error {
	if body, err := io.ReadAll(r.Body); err != nil {
		return fmt.Errorf(`failed to read body: %w`, err)
	} else if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("failed to unmarshal body: %w", err)
	}
	return nil
}
