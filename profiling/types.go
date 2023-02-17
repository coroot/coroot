package profiling

import (
	"github.com/pyroscope-io/pyroscope/pkg/model/appmetadata"
	"github.com/pyroscope-io/pyroscope/pkg/storage/metadata"
	"github.com/pyroscope-io/pyroscope/pkg/structs/flamebearer"
	"strings"
)

type Type string
type Spy string
type View string

const (
	TypeCPU    = "CPU"
	TypeMemory = "Memory"
	TypeOther  = "Other"

	SpyEbpf Spy = "ebpfspy"

	ViewSingle View = "single"
	ViewDiff   View = "diff"
)

type ProfileMeta struct {
	Type  Type   `json:"type"`
	Name  string `json:"name"`
	Query string `json:"query"`
	Spy   Spy    `json:"spy"`
}

type Profile flamebearer.FlamebearerProfileV1

type Metadata []appmetadata.ApplicationMetadata

func (md Metadata) GetApplications() map[string][]ProfileMeta {
	res := map[string][]ProfileMeta{}
	for _, a := range md {
		i := strings.LastIndexByte(a.FQName, '.')
		if i < 0 || i >= len(a.FQName) {
			continue
		}
		app, name := a.FQName[:i], a.FQName[i+1:]
		p := ProfileMeta{Name: name, Query: a.FQName, Spy: Spy(a.SpyName)}
		if p.Spy == SpyEbpf {
			app = ""
			p.Name = "ebpf"
		}
		switch a.Units {
		case metadata.SamplesUnits:
			p.Type = TypeCPU
		case metadata.BytesUnits, metadata.ObjectsUnits:
			p.Type = TypeMemory
		default:
			p.Type = TypeOther
		}
		if strings.ToLower(string(p.Type)) == strings.ToLower(p.Name) {
			p.Name = ""
		}
		res[app] = append(res[app], p)
	}
	return res
}
