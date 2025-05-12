package model

import (
	"fmt"
	"strconv"
	"strings"
)

type Severity int

const (
	SeverityUnknown Severity = iota
	SeverityTrace
	SeverityDebug
	SeverityInfo
	SeverityWarning
	SeverityError
	SeverityFatal
)

func (s Severity) String() string {
	switch s {
	case SeverityUnknown:
		return "unknown"
	case SeverityTrace:
		return "trace"
	case SeverityDebug:
		return "debug"
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityFatal:
		return "fatal"
	}
	return fmt.Sprintf("severity-%d", s)
}

func (s Severity) Color() string {
	switch s {
	case SeverityUnknown:
		return "grey-lighten1"
	case SeverityTrace:
		return "green-lighten4"
	case SeverityDebug:
		return "green-lighten2"
	case SeverityInfo:
		return "blue-lighten2"
	case SeverityWarning:
		return "orange-lighten1"
	case SeverityError:
		return "red-darken1"
	case SeverityFatal:
		return "black"
	}
	return ""
}

func (s Severity) Range() (int, int) {
	switch s {
	case SeverityUnknown:
		return 0, 0
	case SeverityTrace:
		return 1, 4
	case SeverityDebug:
		return 5, 8
	case SeverityInfo:
		return 9, 12
	case SeverityWarning:
		return 13, 16
	case SeverityError:
		return 17, 20
	case SeverityFatal:
		return 21, 24
	}
	return int(s), int(s)
}

func SeverityFromString(s string) Severity {
	switch s {
	case "unknown":
		return SeverityUnknown
	case "trace":
		return SeverityTrace
	case "debug":
		return SeverityDebug
	case "info":
		return SeverityInfo
	case "warn", "warning":
		return SeverityWarning
	case "error":
		return SeverityError
	case "fatal", "critical":
		return SeverityFatal
	}
	i, _ := strconv.Atoi(strings.TrimPrefix(s, "severity-"))
	return Severity(i)
}
