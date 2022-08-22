package model

import "encoding/json"

const (
	UNKNOWN Status = iota
	OK
	INFO
	WARNING
)

type Status int

func (s Status) String() string {
	switch s {
	case OK:
		return "ok"
	case INFO:
		return "info"
	case WARNING:
		return "warning"
	default:
		return "unknown"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
