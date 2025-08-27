package timeseries

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

type Value float32

func (v Value) String() string {
	if IsNaN(float32(v)) {
		return "."
	}
	if v == 0 {
		return "0"
	}
	if v == Value(int(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%f", v)
}

func (v Value) MarshalJSON() ([]byte, error) {
	f := float32(v)
	if IsNaN(f) || IsInf(f, 0) {
		return json.Marshal(nil)
	}
	return json.Marshal(f)
}

func asBytes32[T any](slice []T) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&slice[0])), len(slice)*4)
}

func asFloats32(b []byte) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), len(b)/4)
}
