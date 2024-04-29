package collector

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

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
