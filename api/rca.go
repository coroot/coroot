package api

import (
	"context"
	"net/http"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/cloud"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
)

func (api *Api) RCA(w http.ResponseWriter, r *http.Request, u *db.User) {
	rca := &model.RCA{}
	projectId := db.ProjectId(mux.Vars(r)["project"])
	from, to, incident, _ := api.getTimeContext(r)

	defer func() {
		if incident != nil {
			if err := api.db.UpdateIncidentRCA(projectId, incident, rca); err != nil {
				klog.Errorln(err)
			}
		} else {
			utils.WriteJson(w, rca)
		}
	}()

	cloudAPI := cloud.API(api.db, api.deploymentUuid, api.instanceUuid, r.Referer())
	if status, err := cloudAPI.RCAStatus(r.Context(), false); status != "OK" {
		rca.Status = status
		if err != nil {
			rca.Error = err.Error()
		}
		return
	}

	project, err := api.db.GetProject(projectId)
	if err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}

	if project.Multicluster() {
		klog.Errorln("RCA is not supported for multi-cluster projects")
		rca.Status = "Failed"
		rca.Error = "RCA is not supported for multi-cluster projects"
		return
	}

	appId, err := GetApplicationId(r)
	if err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}
	cacheClient := api.cache.GetCacheClient(project.Id)
	cacheTo, err := cacheClient.GetTo()
	if err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}
	if cacheTo.IsZero() || cacheTo.Before(from) {
		rca.Status = "Failed"
		rca.Error = "Metric cache is empty"
		return
	}
	cacheStep, err := cacheClient.GetStep(from, to)
	if err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}
	if cacheStep == 0 {
		rca.Status = "Failed"
		rca.Error = "Metric cache is empty"
		return
	}
	if cacheTo.Before(to) {
		to = cacheTo
	}
	step := increaseStepForBigDurations(from, to, cacheStep)

	rcaRequest := cloud.RCARequest{
		Ctx:                         timeseries.NewContext(from, to, step),
		ApplicationId:               appId,
		ApplicationCategorySettings: project.Settings.ApplicationCategorySettings,
		CustomApplications:          project.Settings.CustomApplications,
		CustomCloudPricing:          project.Settings.CustomCloudPricing,
	}
	rcaRequest.Ctx.RawStep = cacheStep
	if incident != nil {
		rcaRequest.Ctx.From, rcaRequest.Ctx.To = api.IncidentTimeContext(projectId, incident, to)
	}

	if rcaRequest.CheckConfigs, err = api.db.GetCheckConfigs(project.Id); err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}
	if rcaRequest.ApplicationDeployments, err = api.db.GetApplicationDeployments(project.Id); err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}

	ctr := constructor.New(api.db, project, map[db.ProjectId]constructor.Cache{project.Id: cacheClient}, api.pricing)
	if rcaRequest.Metrics, err = ctr.QueryCache(r.Context(), cacheClient, project, rcaRequest.Ctx.From, rcaRequest.Ctx.To, rcaRequest.Ctx.Step); err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}

	var ch *clickhouse.Client
	if ch, err = api.GetClickhouseClient(project, ""); err != nil {
		klog.Errorln(err)
	}
	if ch != nil {
		rcaRequest.KubernetesEvents, err = ch.GetKubernetesEvents(r.Context(), from, to, 1000)
		if err != nil {
			klog.Errorln(err)
		}

		func() {
			world, _, _, err := api.LoadWorldByRequest(r)
			if err != nil {
				klog.Errorln(err)
				return
			}
			app := world.GetApplication(appId)
			if app == nil {
				return
			}
			rcaRequest.ErrorTrace, rcaRequest.SlowTrace, err = ch.GetTracesViolatingSLOs(r.Context(), rcaRequest.Ctx.From, rcaRequest.Ctx.To, world, app)
			if err != nil {
				klog.Errorln(err)
				return
			}
		}()
	}

	rcaResponse, err := cloudAPI.RCA(r.Context(), rcaRequest)
	if err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}
	rca = rcaResponse
	rca.Status = "OK"
}

func (api *Api) IncidentRCA(ctx context.Context, project *db.Project, world *model.World, incident *model.ApplicationIncident) {
	rca := incident.RCA
	if rca != nil && rca.Status == "OK" {
		return
	}
	if rca == nil {
		rca = &model.RCA{}
	}
	defer func() {
		if err := api.db.UpdateIncidentRCA(project.Id, incident, rca); err != nil {
			klog.Errorln(err)
		}
	}()

	cloudAPI := cloud.API(api.db, api.deploymentUuid, api.instanceUuid, "")
	if status, err := cloudAPI.RCAStatus(ctx, true); status != "OK" {
		rca.Status = status
		if err != nil {
			rca.Error = err.Error()
		}
		return
	}
	if incident.RCA == nil {
		if err := api.db.UpdateIncidentRCA(project.Id, incident, &model.RCA{Status: "In progress"}); err != nil {
			klog.Errorln(err)
			return
		}
	}

	if project.Multicluster() {
		klog.Errorln("RCA is not supported for mult-cluster projects")
		rca.Status = "Failed"
		rca.Error = "RCA is not supported for mult-cluster projects"
		return
	}

	app := world.GetApplication(incident.ApplicationId)
	if app == nil {
		klog.Errorln("application not found")
		rca.Status = "Failed"
		rca.Error = "application not found"
		return
	}

	var err error
	rcaRequest := cloud.RCARequest{
		Ctx:                         world.Ctx,
		ApplicationId:               app.Id,
		CheckConfigs:                world.CheckConfigs,
		ApplicationCategorySettings: project.Settings.ApplicationCategorySettings,
		CustomApplications:          project.Settings.CustomApplications,
		CustomCloudPricing:          project.Settings.CustomCloudPricing,
	}
	rcaRequest.Ctx.From, rcaRequest.Ctx.To = api.IncidentTimeContext(project.Id, incident, world.Ctx.To)

	if rcaRequest.ApplicationDeployments, err = api.db.GetApplicationDeployments(project.Id); err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}

	cacheClient := api.cache.GetCacheClient(project.Id)
	ctr := constructor.New(api.db, project, map[db.ProjectId]constructor.Cache{project.Id: cacheClient}, api.pricing)
	if rcaRequest.Metrics, err = ctr.QueryCache(ctx, cacheClient, project, rcaRequest.Ctx.From, rcaRequest.Ctx.To, rcaRequest.Ctx.Step); err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}

	var ch *clickhouse.Client
	if ch, err = api.GetClickhouseClient(project, ""); err != nil {
		klog.Errorln(err)
	}
	if ch != nil {
		rcaRequest.KubernetesEvents, err = ch.GetKubernetesEvents(ctx, rcaRequest.Ctx.From, rcaRequest.Ctx.To, 1000)
		if err != nil {
			klog.Errorln(err)
		}
		rcaRequest.ErrorTrace, rcaRequest.SlowTrace, err = ch.GetTracesViolatingSLOs(ctx, rcaRequest.Ctx.From, rcaRequest.Ctx.To, world, app)
		if err != nil {
			klog.Errorln(err)
		}
	}

	rcaResponse, err := cloudAPI.RCA(ctx, rcaRequest)
	if err != nil {
		klog.Errorln(err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return
	}
	rca = rcaResponse
	rca.Status = "OK"
}

func (api *Api) IncidentTimeContext(projectId db.ProjectId, incident *model.ApplicationIncident, now timeseries.Time) (timeseries.Time, timeseries.Time) {
	from := incident.OpenedAt.Add(-model.IncidentTimeOffset)
	to := now
	if incident.Resolved() {
		to = incident.ResolvedAt
	}
	incidents, err := api.db.GetApplicationIncidents(projectId, from, incident.OpenedAt)
	if err != nil {
		klog.Errorln(err)
		return from, to
	}
	for _, i := range incidents[incident.ApplicationId] {
		if i.Key == incident.Key || !i.Resolved() {
			continue
		}
		if i.ResolvedAt.After(from) && i.ResolvedAt.Before(to) {
			from = i.ResolvedAt
		}
	}
	return from, to
}
