package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rca/llm"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

const (
	// Upstream re-runs RCA for any non-OK incident on every cache cycle. Against Coroot Cloud that is
	// gated server-side; a self-hosted LLM needs its own ceiling.
	rcaMaxAutoAttempts = 3
	// Bounds an incident storm: excess incidents are picked up on a later watcher cycle.
	rcaMaxConcurrentInvestigations = 2

	rcaKubernetesEventsLimit = 1000
)

// rcaRunner owns the local LLM client and bounds automatic investigations.
type rcaRunner struct {
	cfg    config.RCA
	client *llm.Client

	lock     sync.Mutex
	inFlight map[string]bool
	attempts map[string]int
	slots    chan struct{}
}

func newRCARunner(cfg config.RCA) *rcaRunner {
	r := &rcaRunner{
		inFlight: map[string]bool{},
		attempts: map[string]int{},
		slots:    make(chan struct{}, rcaMaxConcurrentInvestigations),
	}
	r.update(cfg)
	return r
}

func (r *rcaRunner) update(cfg config.RCA) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.cfg = cfg
	if cfg.IsLocal() {
		r.client = llm.NewClient(llm.Config{
			BaseUrl:      cfg.BaseUrl,
			ApiKey:       cfg.ApiKey,
			Model:        cfg.Model,
			SystemPrompt: cfg.SystemPrompt,
			Timeout:      cfg.Timeout.ToStandard(),
		})
		return
	}
	r.client = nil
}

func (r *rcaRunner) config() config.RCA {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.cfg
}

func (r *rcaRunner) enabled() bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.cfg.IsLocal()
}

func (r *rcaRunner) shouldAutoInvestigate() bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.cfg.IsLocal() && r.cfg.AutoInvestigate
}

func (r *rcaRunner) timeout() time.Duration {
	r.lock.Lock()
	defer r.lock.Unlock()
	d := r.cfg.Timeout.ToStandard()
	if d <= 0 {
		return 5 * time.Minute
	}
	return d
}

func (r *rcaRunner) llm() *llm.Client {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.client
}

func (api *Api) LocalLLM() *llm.Client {
	if api == nil || api.rca == nil {
		return nil
	}
	return api.rca.llm()
}

// begin reserves capacity for an automatic investigation of the given incident. It returns false when
// the incident is already being investigated, has failed too often, or all slots are busy.
func (r *rcaRunner) begin(key string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.inFlight[key] || r.attempts[key] >= rcaMaxAutoAttempts {
		return false
	}
	select {
	case r.slots <- struct{}{}:
	default:
		return false
	}
	r.inFlight[key] = true
	r.attempts[key]++
	return true
}

func (r *rcaRunner) done(key string, succeeded bool) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.inFlight, key)
	if succeeded {
		delete(r.attempts, key)
	}
	<-r.slots
}

func (api *Api) localRCAEnabled() bool {
	return api.rca != nil && api.rca.enabled()
}

// localRCA runs an on-demand investigation. It loads and audits the world itself because the caller
// only has the HTTP request context.
func (api *Api) localRCA(ctx context.Context, project *db.Project, appId model.ApplicationId, incident *model.ApplicationIncident, from, to timeseries.Time) *model.RCA {
	rca := &model.RCA{}
	if project.Multicluster() {
		rca.Status = "Failed"
		rca.Error = "RCA is not supported for multi-cluster projects"
		return rca
	}

	world, _, err := api.LoadWorld(ctx, project, from, to)
	if err != nil {
		klog.Errorln("rca: failed to load world:", err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return rca
	}
	if world == nil {
		rca.Status = "Failed"
		rca.Error = "No data available"
		return rca
	}
	app := world.GetApplication(appId)
	if app == nil {
		rca.Status = "Failed"
		rca.Error = "Application not found"
		return rca
	}
	auditor.Audit(world, project, app, nil)

	result, err := api.investigateLocally(ctx, project, world, app, incident, from, to)
	if err != nil {
		klog.Errorln("rca:", err)
		rca.Status = "Failed"
		rca.Error = err.Error()
		return rca
	}
	return result
}

// localIncidentRCA starts an automatic investigation in the background. The incident watcher calls
// this synchronously before enqueuing notifications, so it must return immediately.
func (api *Api) localIncidentRCA(project *db.Project, world *model.World, incident *model.ApplicationIncident) {
	if !api.rca.shouldAutoInvestigate() || project.Multicluster() {
		return
	}
	app := world.GetApplication(incident.ApplicationId)
	if app == nil {
		return
	}
	if !api.rca.begin(incident.Key) {
		return
	}

	// The world is already audited by the watcher, so it can be summarized as-is.
	from, to := api.IncidentTimeContext(project.Id, incident, world.Ctx.To)

	// Surfaces the spinner in the incident view for the duration of the analysis, including retries.
	if err := api.db.UpdateIncidentRCA(project.Id, incident, &model.RCA{Status: "In progress"}); err != nil {
		klog.Errorln("rca: failed to mark incident as in progress:", err)
		api.rca.done(incident.Key, false)
		return
	}

	// The watcher keeps using this incident (notifications) while we work, and UpdateIncidentRCA
	// writes back into it, so the background goroutine operates on its own copy.
	background := *incident

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), api.rca.timeout())
		defer cancel()

		rca, err := api.investigateLocally(ctx, project, world, app, &background, from, to)
		if err != nil {
			klog.Errorf("rca: incident %s: %s", background.Key, err)
			rca = &model.RCA{Status: "Failed", Error: err.Error()}
		}
		api.rca.done(background.Key, err == nil)
		if err := api.db.UpdateIncidentRCA(project.Id, &background, rca); err != nil {
			klog.Errorln("rca: failed to save result:", err)
		}
	}()
}

// investigateLocally summarizes the telemetry for the application and asks the configured LLM for a
// root cause. The world must already be audited.
func (api *Api) investigateLocally(ctx context.Context, project *db.Project, world *model.World, app *model.Application, incident *model.ApplicationIncident, from, to timeseries.Time) (*model.RCA, error) {
	client := api.rca.llm()
	if client == nil {
		return nil, fmt.Errorf("no LLM configured for local RCA")
	}

	in := rcaEvidenceInput{
		world:    world,
		app:      app,
		incident: incident,
		from:     from,
		to:       to,
	}

	// Deployments are useful but not essential; a missing rollout history shouldn't block the analysis.
	if deployments, err := api.db.GetApplicationDeployments(project.Id); err != nil {
		klog.Errorln("rca: failed to get deployments:", err)
	} else {
		in.deployments = deployments[app.Id]
	}

	ch, err := api.GetClickhouseClient(project, "")
	if err != nil {
		klog.Errorln("rca: failed to get clickhouse client:", err)
	}
	if ch != nil {
		defer ch.Close()
		if in.kubernetesEvents, err = ch.GetKubernetesEvents(ctx, from, to, rcaKubernetesEventsLimit); err != nil {
			klog.Errorln("rca: failed to get kubernetes events:", err)
		}
		if in.errorTrace, in.slowTrace, err = ch.GetTracesViolatingSLOs(ctx, from, to, world, app); err != nil {
			klog.Errorln("rca: failed to get traces:", err)
		}
	}

	start := time.Now()
	result, err := client.Analyze(ctx, buildRCAEvidence(in))
	if err != nil {
		return nil, err
	}
	klog.Infof("rca: analyzed %s in %s", app.Id, time.Since(start).Truncate(time.Millisecond))

	return &model.RCA{
		Status:            "OK",
		ShortSummary:      result.ShortSummary,
		RootCause:         result.RootCause,
		ImmediateFixes:    result.ImmediateFixes,
		DetailedRootCause: result.DetailedRootCause,
	}, nil
}
