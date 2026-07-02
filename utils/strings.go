package utils

import "unicode/utf8"

func TruncateUtf8(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	for maxLength > 0 && !utf8.RuneStart(s[maxLength]) {
		maxLength--
	}
	if maxLength == 0 {
		return ""
	}
	return s[:maxLength] + "..."
}
