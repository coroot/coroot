package api

import (
	"testing"

	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/timeseries"
)

func localRCAConfig() config.RCA {
	return config.RCA{
		Provider: config.RCAProviderLocal,
		BaseUrl:  "http://llm.internal/v1",
		Model:    "test/model",
		Timeout:  timeseries.Minute,
	}
}

func TestRCARunnerOnlyBuildsClientForLocalProvider(t *testing.T) {
	if r := newRCARunner(config.RCA{Provider: config.RCAProviderCloud}); r.client != nil {
		t.Error("the cloud provider must not build a local LLM client")
	}
	if r := newRCARunner(config.RCA{}); r.client != nil {
		t.Error("an unset provider must not build a local LLM client")
	}
	if r := newRCARunner(localRCAConfig()); r.client == nil {
		t.Error("the local provider must build an LLM client")
	}
}

func TestRCARunnerDedupesInFlightIncidents(t *testing.T) {
	r := newRCARunner(localRCAConfig())

	if !r.begin("incident-1") {
		t.Fatal("the first investigation should start")
	}
	if r.begin("incident-1") {
		t.Error("the same incident must not be investigated twice concurrently")
	}

	r.done("incident-1", false)

	if !r.begin("incident-1") {
		t.Error("a retry should be allowed once the previous attempt finished")
	}
}

func TestRCARunnerCapsAttempts(t *testing.T) {
	r := newRCARunner(localRCAConfig())

	for i := 0; i < rcaMaxAutoAttempts; i++ {
		if !r.begin("incident-1") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		r.done("incident-1", false)
	}

	if r.begin("incident-1") {
		t.Errorf("an incident must not be retried more than %d times", rcaMaxAutoAttempts)
	}
	if !r.begin("incident-2") {
		t.Error("the cap must be per incident")
	}
}

func TestRCARunnerForgetsAttemptsAfterSuccess(t *testing.T) {
	r := newRCARunner(localRCAConfig())

	if !r.begin("incident-1") {
		t.Fatal("the first investigation should start")
	}
	r.done("incident-1", true)

	if r.attempts["incident-1"] != 0 {
		t.Errorf("a successful investigation should reset the attempt count, got %d", r.attempts["incident-1"])
	}
	if !r.begin("incident-1") {
		t.Error("a re-opened incident should be investigable again")
	}
}

func TestRCARunnerBoundsConcurrency(t *testing.T) {
	r := newRCARunner(localRCAConfig())

	for i := 0; i < rcaMaxConcurrentInvestigations; i++ {
		if !r.begin(string(rune('a' + i))) {
			t.Fatalf("investigation %d should start", i+1)
		}
	}
	if r.begin("overflow") {
		t.Errorf("no more than %d investigations should run at once", rcaMaxConcurrentInvestigations)
	}

	r.done("a", true)

	if !r.begin("overflow") {
		t.Error("a freed slot should admit the next investigation")
	}
}

func TestRCARunnerReleasesSlotsOnFailure(t *testing.T) {
	r := newRCARunner(localRCAConfig())

	// A repeatedly failing incident must not leak concurrency slots.
	for i := 0; i < rcaMaxAutoAttempts; i++ {
		if !r.begin("flaky") {
			t.Fatalf("attempt %d should start", i+1)
		}
		r.done("flaky", false)
	}
	if len(r.slots) != 0 {
		t.Errorf("expected all slots to be released, got %d held", len(r.slots))
	}
	for i := 0; i < rcaMaxConcurrentInvestigations; i++ {
		if !r.begin(string(rune('a' + i))) {
			t.Errorf("slot %d should be available", i+1)
		}
	}
}
