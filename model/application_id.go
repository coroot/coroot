package model

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ApplicationIdZero = ApplicationId{}
	hexPattern        = regexp.MustCompile(`[\da-f]+`)
)

type ApplicationId struct {
	Namespace string
	Kind      ApplicationKind
	Name      string
}

func NewApplicationId(ns string, kind ApplicationKind, name string) ApplicationId {
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
	if ns == "" {
		ns = "_"
	}
	return ApplicationId{Namespace: ns, Kind: kind, Name: name}
}

func NewApplicationIdFromString(src string) (ApplicationId, error) {
	parts := strings.SplitN(src, ":", 3)
	if len(parts) < 3 {
		return ApplicationId{}, fmt.Errorf("invalid application id: %s", src)
	}
	return ApplicationId{Namespace: parts[0], Kind: ApplicationKind(parts[1]), Name: parts[2]}, nil
}

func (a ApplicationId) IsZero() bool {
	return a == ApplicationIdZero
}

func (a ApplicationId) String() string {
	return fmt.Sprintf("%s:%s:%s", a.Namespace, a.Kind, a.Name)
}

func (a ApplicationId) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *ApplicationId) UnmarshalText(text []byte) error {
	var err error
	*a, err = NewApplicationIdFromString(string(text))
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
	*a, err = NewApplicationIdFromString(src.(string))
	return err
}
