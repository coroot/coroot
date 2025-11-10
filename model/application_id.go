package model

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	ClusterIdExternal = "external"
)

var (
	ApplicationIdZero = ApplicationId{}
	hexPattern        = regexp.MustCompile(`[\da-f]+`)
)

type ApplicationId struct {
	ClusterId string
	Namespace string
	Kind      ApplicationKind
	Name      string
}

func NewApplicationId(clusterId, ns string, kind ApplicationKind, name string) ApplicationId {
	switch kind {
	case ApplicationKindReplicaSet:
		parts := strings.Split(name, "-")
		if hexPattern.MatchString(parts[len(parts)-1]) {
			kind = ApplicationKindDeployment
			name = strings.Join(parts[:len(parts)-1], "-")
		}
	case ApplicationKindJob:
		parts := strings.Split(name, "-")
		if _, err := strconv.ParseUint(parts[len(parts)-1], 10, 64); err == nil {
			kind = ApplicationKindCronJob
			name = strings.Join(parts[:len(parts)-1], "-")
		}
	case "", "<none>":
		kind = ApplicationKindPod
	}
	if clusterId == "" {
		clusterId = "_"
	}
	if ns == "" {
		ns = "_"
	}
	return ApplicationId{ClusterId: clusterId, Namespace: ns, Kind: kind, Name: name}
}

func NewApplicationIdFromString(src string, fallbackClusterId string) (ApplicationId, error) {
	parts := strings.SplitN(src, ":", 4)
	var id ApplicationId

	switch len(parts) {
	case 3: // without cluster_id
		id = ApplicationId{ClusterId: "", Namespace: parts[0], Kind: ApplicationKind(parts[1]), Name: parts[2]}
	case 4:
		id = ApplicationId{ClusterId: parts[0], Namespace: parts[1], Kind: ApplicationKind(parts[2]), Name: parts[3]}
		if id.ClusterId == "external" && id.Namespace != "external" { // without cluster_id with ':' in the name
			id = ApplicationId{ClusterId: "", Namespace: parts[0], Kind: ApplicationKind(parts[1]), Name: parts[2] + ":" + parts[3]}
		}
	default:
		return ApplicationId{}, fmt.Errorf("invalid application id: %s", src)
	}
	if id.ClusterId == "" {
		id.ClusterId = fallbackClusterId
	}
	if id.Kind == ApplicationKindExternalService {
		id.ClusterId = ClusterIdExternal
	}
	return id, nil
}

func (a ApplicationId) NamespaceIsEmpty() bool {
	return a.Namespace == "" || a.Namespace == "_"
}

func (a ApplicationId) IsZero() bool {
	return a == ApplicationIdZero
}

func (a ApplicationId) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", a.ClusterId, a.Namespace, a.Kind, a.Name)
}

func (a ApplicationId) StringWithoutClusterId() string {
	return fmt.Sprintf("%s:%s:%s", a.Namespace, a.Kind, a.Name)
}

func (a ApplicationId) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *ApplicationId) UnmarshalText(text []byte) error {
	var err error
	*a, err = NewApplicationIdFromString(string(text), "")
	return err
}

func (a ApplicationId) Value() (driver.Value, error) {
	return a.String(), nil
}

func (a *ApplicationId) Scan(src any) error {
	if src == nil {
		*a = ApplicationId{}
		return nil
	}
	var err error
	*a, err = NewApplicationIdFromString(src.(string), "")
	return err
}
