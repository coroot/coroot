package utils

import (
	"regexp"
)

var (
	imageRegexp = regexp.MustCompile(`(.+@sha256:)([0-9A-Fa-f]{7})[0-9A-Fa-f]{57}`)
)

func FormatImage(orig string) string {
	return imageRegexp.ReplaceAllString(LastPart(orig, "/"), "$1$2")
}
