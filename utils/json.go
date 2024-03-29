package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/buger/jsonparser"

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

func EscapeJsonMultilineStrings(data []byte) []byte {
	var offsets []int
	err := jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		if dataType == jsonparser.String {
			for i, c := range value {
				if c == '\n' {
					o := offset - len(value) + i - 1
					offsets = append(offsets, o)
				}
			}
		}
		return nil
	})
	if err != nil { // not a JSON
		return data
	}

	if len(offsets) == 0 {
		return data
	}

	b := bytes.NewBuffer(nil)
	var from int
	for o := range offsets {
		to := offsets[o]
		b.Write(data[from:to])
		b.Write([]byte("\\n"))
		from = to + 1
	}
	b.Write(data[from:])

	return b.Bytes()
}
