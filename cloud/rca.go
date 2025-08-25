package cloud

import (
	"bytes"
	"context"
	"net/http"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/vmihailenco/msgpack/v5"
)

type RCARequest struct {
	Ctx timeseries.Context

	ApplicationId model.ApplicationId

	CheckConfigs                model.CheckConfigs
	ApplicationDeployments      map[model.ApplicationId][]*model.ApplicationDeployment
	ApplicationCategorySettings map[model.ApplicationCategory]*db.ApplicationCategorySettings
	CustomApplications          map[string]model.CustomApplication
	CustomCloudPricing          *db.CustomCloudPricing

	Metrics map[string][]*model.MetricValues

	KubernetesEvents []*model.LogEntry
}

func (api *Api) RCA(ctx context.Context, req RCARequest) (*model.RCA, error) {
	buf := bytes.NewBuffer(nil)
	err := msgpack.NewEncoder(buf).Encode(req)
	if err != nil {
		return nil, err
	}

	var rca model.RCA
	err = api.request(ctx, http.MethodPost, "/integration/rca", buf, &rca)
	if err != nil {
		return nil, err
	}
	return &rca, nil
}

func (api *Api) RCAStatus(ctx context.Context, incidentsAutoInvestigation bool) (string, error) {
	settings, err := api.GetSettings()
	if err != nil {
		return "Failed", err
	}
	if settings.ApiKey == "" {
		return "AI disabled", nil
	}
	if incidentsAutoInvestigation && settings.RCA.DisableIncidentsAutoInvestigation {
		return "AI disabled", nil
	}
	info, err := api.IntegrationInfo(ctx)
	if err != nil {
		return "Failed", err
	}
	if info.RCA == nil {
		return "AI disabled", nil
	}
	if info.RCA.CreditsSpent >= info.RCA.CreditsTotal {
		return "Out of credits", err
	}
	return "OK", nil
}
