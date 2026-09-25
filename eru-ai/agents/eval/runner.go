package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

// Runner drives a fixture against a running stack and scores what came back.
//
// It calls the same endpoint the product calls - no test-only path into the
// agent - so a run scored here is a run a user could have had. That matters:
// several of the defects this suite exists for were in the wiring around the
// agent, not in the agent, and a harness that bypassed the wiring would have
// reported them all green.
type Runner struct {
	BaseURL string // eru-ai, e.g. http://localhost:8088
	Project string // "processo"
	Tenant  string // the org_process_id the run belongs to
	Claims  string // the claims header the gateway normally supplies
	Client  *http.Client

	// Reading the workspace back needs eru-ql and the org/process the tenant
	// maps to. Left unset, outcome assertions are skipped rather than failed:
	// a check that cannot run must not read as a check that failed.
	EruqlURL  string
	OrgId     string
	ProcessId string

	// DefaultSamples applies to fixtures that do not set their own. One is a
	// measurement of nothing, so anything drawing a conclusion should raise it.
	DefaultSamples int
}

// RunnerFromEnv reads the stack's coordinates from the environment, so the
// suite carries no tenant of its own. Reports what is missing rather than
// failing obscurely halfway through a ten-minute run.
func RunnerFromEnv() (*Runner, error) {
	runner := &Runner{
		BaseURL: envOr("ERU_EVAL_BASE_URL", "http://localhost:8088"),
		Project: envOr("ERU_EVAL_PROJECT", "processo"),
		Tenant:  os.Getenv("ERU_EVAL_TENANT"),
		Claims:  os.Getenv("ERU_EVAL_CLAIMS"),
		// An orchestrated build runs for minutes: three sub-agents and a page
		// compose took twenty on the slowest run seen.
		Client:         &http.Client{Timeout: 45 * time.Minute},
		EruqlURL:       envOr("ERU_EVAL_ERUQL_URL", "http://localhost:8087"),
		OrgId:          os.Getenv("ERU_EVAL_ORG_ID"),
		ProcessId:      os.Getenv("ERU_EVAL_PROCESS_ID"),
		DefaultSamples: 1,
	}
	if runner.Tenant == "" {
		return nil, fmt.Errorf("live eval needs ERU_EVAL_TENANT")
	}

	// Claims can be handed over directly, or earned by logging in. Logging in is
	// preferred: a pasted header is a credential someone has to keep fresh, and
	// a stale one fails as an empty result rather than as an error.
	if runner.Claims == "" {
		user, password := os.Getenv("ERU_EVAL_USERNAME"), os.Getenv("ERU_EVAL_PASSWORD")
		if user == "" || password == "" {
			return nil, fmt.Errorf("live eval needs ERU_EVAL_CLAIMS, or ERU_EVAL_USERNAME and ERU_EVAL_PASSWORD to obtain it")
		}
		claims, err := Login(context.Background(), &http.Client{Timeout: 60 * time.Second},
			envOr("ERU_EVAL_AUTH_URL", "https://eruauth.dev.processo.io"), runner.Project, user, password)
		if err != nil {
			return nil, err
		}
		runner.Claims = claims
	}
	return runner, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Run executes one fixture and reduces the reply to a trajectory.
//
// raw=true is required: without it the traces never reach the caller and every
// assertion about what the agent DID would have nothing to read.
func (r *Runner) Run(ctx context.Context, fixture Fixture) (Trajectory, error) {
	return r.RunAttempt(ctx, fixture, 1)
}

// RunAttempt is Run for a numbered attempt, so a fixture whose prompt carries
// "{{sample}}" writes to a different target each time.
func (r *Runner) RunAttempt(ctx context.Context, fixture Fixture, attempt int) (Trajectory, error) {
	agent := fixture.Agent
	if agent == "" {
		agent = "processo"
	}
	url := fmt.Sprintf("%s/%s/%s/execute/agent/%s?raw=true", strings.TrimRight(r.BaseURL, "/"), r.Project, r.Tenant, agent)

	body, err := json.Marshal(agents.AgentMessage{
		Content: fixture.PromptFor(attempt),
		Params:  map[string]interface{}{"output_mode": "auto"},
	})
	if err != nil {
		return Trajectory{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Trajectory{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("claims", r.Claims)

	started := time.Now()
	response, err := r.Client.Do(request)
	if err != nil {
		return Trajectory{}, fmt.Errorf("%s: %w", fixture.Name, err)
	}
	defer response.Body.Close()

	var reply agents.AgentMessage
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		return Trajectory{}, fmt.Errorf("%s: decoding the reply: %w", fixture.Name, err)
	}
	if response.StatusCode >= 300 {
		return Trajectory{}, fmt.Errorf("%s: agent answered %d", fixture.Name, response.StatusCode)
	}

	traj := FromRun(fixture.PromptFor(attempt), reply)
	traj.Agent = agent
	traj.DurationMs = time.Since(started).Milliseconds()
	if len(traj.ToolCalls) == 0 && len(traj.Actions) == 0 {
		return traj, fmt.Errorf("%s: the reply carried no traces and no actions - was raw=true honoured?", fixture.Name)
	}
	return traj, nil
}

// RunAll executes a suite in order and scores each fixture.
//
// Sequential on purpose. These runs write to a real workspace, and two builds
// racing on the same entity list would score each other's work.
func (r *Runner) RunAll(ctx context.Context, suite Suite, record string) ([]Result, error) {
	var results []Result
	for _, fixture := range suite {
		// Snapshot first: "this run added an entity for addresses" cannot be told
		// from "an entity for addresses has existed since last year" without it.
		var before Outcome
		if len(fixture.ExpectOutcome) > 0 && r.OrgId != "" && r.ProcessId != "" {
			if snapshot, err := r.ReadOutcome(ctx, r.EruqlURL, r.OrgId, r.ProcessId); err == nil {
				before = snapshot
			}
		}
		traj, err := r.Run(ctx, fixture)
		if err != nil {
			results = append(results, Result{
				Fixture:  fixture.Name,
				Failures: []Failure{{Expectation: "the run completes", Reason: err.Error()}},
			})
			continue
		}
		if record != "" {
			if err := traj.Save(record, fixture.Name); err != nil {
				return results, err
			}
		}
		result := fixture.Score(traj)
		if len(fixture.ExpectOutcome) > 0 {
			if r.OrgId == "" || r.ProcessId == "" {
				result.Failures = append(result.Failures, Failure{
					Expectation: "the workspace can be read back",
					Reason:      "ERU_EVAL_ORG_ID and ERU_EVAL_PROCESS_ID are needed to check the outcome",
				})
			} else {
				after, err := r.ReadOutcome(ctx, r.EruqlURL, r.OrgId, r.ProcessId)
				if err != nil {
					result.Failures = append(result.Failures, Failure{
						Expectation: "the workspace can be read back", Reason: err.Error(),
					})
				} else {
					result = fixture.ScoreOutcome(result, before, after)
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// infraFailure decides whether an error means "the agent did badly" or "the
// attempt never happened".
//
// The distinction is not pedantry. A stack that was down for two attempts out of
// three, scored as two failures, reads as a 33% pass rate and sends someone
// looking for a defect in the agent. HarnessX counts infrastructure failures AS
// failures deliberately, on the grounds that an agent which cannot run is an
// agent that did not solve the task; DarwinX separates them by policy and retries
// only what is outside the agent's control. Either is defensible. Conflating them
// silently, which is what we did until now, is not.
//
// We separate them, and an attempt that never ran is excluded from the rate
// rather than counted as a pass - so a fixture whose every attempt failed on
// infrastructure is BLOCKED, never green.
//
// A timeout is deliberately NOT infrastructure. An agent that runs past its
// budget is an agent that failed the task, which is DarwinX's rule and the right
// one: it is the failure mode most likely to be caused by the change under test.
func infraFailure(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	lower := strings.ToLower(text)
	for _, signature := range []string{
		"connection refused",
		"no such host",
		"connection reset",
		"eof",
		"broken pipe",
		"network is unreachable",
		"tls handshake",
		"agent answered 5",   // the service failed, not the agent
		"agent answered 401", // the claims header expired mid-suite
		"agent answered 403",
	} {
		if strings.Contains(lower, signature) {
			return text
		}
	}
	return ""
}

// RunAllSampled runs every fixture `samples` times and reports the spread.
//
// Sequential, and samples of the same fixture run back to back: these write to a
// real workspace, and two builds racing on one entity list would score each
// other's work.
//
// Note what repeating does to an outcome assertion. The first sample of a
// "builds something new" fixture leaves the workspace holding what it built, so
// the second sample finds it already there - which is exactly the RequiresAbsent
// case, and it is reported BLOCKED rather than failed. That is honest, and it is
// why such fixtures want a workspace reset between samples rather than more
// samples.
func (r *Runner) RunAllSampled(ctx context.Context, suite Suite, record string, override int) ([]SampledResult, error) {
	var out []SampledResult
	for _, fixture := range suite {
		samples := fixture.SampleCount(r.DefaultSamples, override)
		sampled := SampledResult{Fixture: fixture.Name}
		for i := 0; i < samples; i++ {
			result, infra, traj, err := r.runOnce(ctx, fixture, record, i, samples)
			if err != nil {
				return out, err
			}
			sampled.Attempts = append(sampled.Attempts, Attempt{Result: result, Infra: infra, Trajectory: traj})
			// A fixture that cannot be judged cannot be judged k times either,
			// and repeating it only burns minutes.
			if result.Blocked != "" {
				sampled.Blocked = result.Blocked
				break
			}
		}
		out = append(out, sampled)
	}
	return out, nil
}

// runOnce is one attempt: snapshot, run, score. It returns the score, or the
// reason the attempt never happened.
func (r *Runner) runOnce(ctx context.Context, fixture Fixture, record string, index, samples int) (Result, string, Trajectory, error) {
	// The snapshot is needed for outcome assertions AND for preconditions, and
	// the preconditions matter even to a fixture that asserts nothing about the
	// workspace: both attachment scenarios assert only on actions, and both were
	// meaningless the day their entity was deleted.
	needsWorkspace := fixture.WantsOutcome() ||
		len(fixture.RequiresPresent) > 0 || len(fixture.RequiresAbsent) > 0
	var before Outcome
	if needsWorkspace && r.OrgId != "" && r.ProcessId != "" {
		if snapshot, err := r.ReadOutcome(ctx, r.EruqlURL, r.OrgId, r.ProcessId); err == nil {
			before = snapshot
		}
	}

	// Check preconditions BEFORE spending a run on a scenario that cannot mean
	// anything. Ten minutes of dashboard build is a poor way to discover that
	// the entity it needed was missing all along.
	if precondition := fixture.CheckPreconditions(before); precondition != "" {
		return Result{Fixture: fixture.Name, Blocked: precondition}, "", Trajectory{}, nil
	}

	traj, err := r.RunAttempt(ctx, fixture, index+1)
	if err != nil {
		if reason := infraFailure(err); reason != "" {
			return Result{Fixture: fixture.Name}, reason, Trajectory{}, nil
		}
		return Result{
			Fixture:  fixture.Name,
			Failures: []Failure{{Expectation: "the run completes", Reason: err.Error()}},
		}, "", Trajectory{}, nil
	}

	result := fixture.Score(traj)
	if fixture.WantsOutcome() {
		if r.OrgId == "" || r.ProcessId == "" {
			result.Failures = append(result.Failures, Failure{
				Expectation: "the workspace can be read back",
				Reason:      "ERU_EVAL_ORG_ID and ERU_EVAL_PROCESS_ID are needed to check the outcome",
			})
		} else if after, err := r.ReadOutcome(ctx, r.EruqlURL, r.OrgId, r.ProcessId); err != nil {
			result.Failures = append(result.Failures, Failure{
				Expectation: "the workspace can be read back", Reason: err.Error(),
			})
		} else {
			result = fixture.ScoreOutcomeFor(result, before, after, index+1)
		}
	}

	// Recorded AFTER scoring, and a failing run is recorded under a name the
	// replay suite will not match.
	//
	// A recording is the offline regression baseline: replayed by
	// TestRecordedScenarios on every `go test ./...`, with no stack and no cost.
	// Saving a FAILING run as that baseline makes the offline suite permanently
	// red for a reason that has nothing to do with the change being tested - and
	// a suite that is always red is a suite nobody reads. It happened the first
	// time the quality gate shipped a page below the bar: the run was recorded,
	// and every subsequent `go test` reported that failure again.
	//
	// The failing trajectory is still written, because it is the best evidence
	// there is about what went wrong. It just is not the baseline.
	if record != "" {
		name := fixture.Name
		if samples > 1 {
			name = fmt.Sprintf("%s.sample%d", fixture.Name, index+1)
		}
		if len(result.Failures) > 0 {
			name += ".failed"
		}
		if err := traj.Save(record, name); err != nil {
			return Result{}, "", Trajectory{}, err
		}
	}
	return result, "", traj, nil
}
