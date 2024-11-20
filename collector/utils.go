package collector

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
	v1 "go.opentelemetry.io/proto/otlp/common/v1"
)

func attributesToMap(kv []*v1.KeyValue) map[string]string {
	res := make(map[string]string, len(kv))
	for _, attr := range kv {
		res[attr.GetKey()] = valueToString(attr.GetValue())
	}
	return res
}

func valueToString(value *v1.AnyValue) string {
	switch value.Value.(type) {
	case *v1.AnyValue_StringValue:
		return value.GetStringValue()
	case *v1.AnyValue_BoolValue:
		return strconv.FormatBool(value.GetBoolValue())
	case *v1.AnyValue_IntValue:
		return strconv.FormatInt(value.GetIntValue(), 10)
	case *v1.AnyValue_DoubleValue:
		v := value.GetDoubleValue()
		if math.IsNaN(v) {
			return "NaN"
		}
		if math.IsInf(v, 1) {
			return "+Inf"
		}
		if math.IsInf(v, -1) {
			return "-Inf"
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case *v1.AnyValue_BytesValue:
		return string(value.GetBytesValue())
	case *v1.AnyValue_ArrayValue:
		vs := value.GetArrayValue().GetValues()
		s := make([]string, 0, len(vs))
		for _, v := range vs {
			s = append(s, valueToString(v))
		}
		j, _ := json.Marshal(s)
		return string(j)
	case *v1.AnyValue_KvlistValue:
		vs := value.GetKvlistValue().Values
		s := make(map[string]string, len(vs))
		for _, kv := range vs {
			s[kv.GetKey()] = valueToString(kv.GetValue())
		}
		j, _ := json.Marshal(s)
		return string(j)
	}
	return fmt.Sprintf("unknown attribute value type: %T", value.Value)
}

func getDecoder(encoding string, body io.ReadCloser) (io.ReadCloser, error) {
	switch encoding {
	case "", "node":
		return body, nil
	case "gzip":
		return gzip.NewReader(body)
	case "zlib", "deflate":
		return zlib.NewReader(body)
	case "zstd":
		r, err := zstd.NewReader(body, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		return r.IOReadCloser(), nil
	case "snappy":
		r := snappy.NewReader(body)
		b := new(bytes.Buffer)
		_, err := io.Copy(b, r)
		if err != nil {
			return nil, err
		}
		if err = body.Close(); err != nil {
			return nil, err
		}
		return io.NopCloser(b), nil
	}
	return nil, fmt.Errorf("unsupported content encoding: %q", encoding)
}
