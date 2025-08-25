package cloud

import (
	"context"
	"net/http"
)

type IntegrationInfo struct {
	RCA *IntegrationInfoRCA `json:"rca"`
}

type IntegrationInfoRCA struct {
	CreditsTotal int    `json:"credits_total"`
	CreditsSpent int    `json:"credits_spent"`
	RenewsAt     int64  `json:"renews_at"`
	Plan         string `json:"plan"`
	Price        int64  `json:"price"`
	Currency     string `json:"currency"`
	Interval     string `json:"interval"`
}

func (api *Api) IntegrationInfo(ctx context.Context) (*IntegrationInfo, error) {
	var info IntegrationInfo
	err := api.request(ctx, http.MethodGet, "/integration/info", nil, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
