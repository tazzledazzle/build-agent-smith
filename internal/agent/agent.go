// Package agent orchestrates parallel audit workers under supervisor plans.
package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/tazzledazzle/build-agent-smith/internal/aggregator"
	"github.com/tazzledazzle/build-agent-smith/internal/cicd"
	"github.com/tazzledazzle/build-agent-smith/internal/coverage"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/finops"
	"github.com/tazzledazzle/build-agent-smith/internal/reposcanner"
	"github.com/tazzledazzle/build-agent-smith/internal/supervisor"
)

// CoverageInput is coverage data supplied by an external fetcher.
type CoverageInput struct {
	LinePct        float64
	BranchPct      float64
	CommitsPerWeek float64
}

// Dependencies abstracts external data sources for the audit agent.
type Dependencies interface {
	HasFile(ctx context.Context, owner, repo, path string) (bool, error)
	FetchPipelineYAML(ctx context.Context, owner, repo string) (string, error)
	FetchCoverage(ctx context.Context, repo string) (CoverageInput, error)
	FetchCloudInventory(ctx context.Context) (finops.Inventory, error)
}

// RunRequest triggers an audit cycle.
type RunRequest struct {
	Scope          domain.Scope
	Repos          []domain.RepoConfig
	TargetRepoName string
}

// Agent is the LangGraph-equivalent orchestrator for platform audits.
type Agent struct {
	deps    Dependencies
	scanner *reposcanner.Scanner
}

// New creates an Agent with the given data-source dependencies.
func New(deps Dependencies) *Agent {
	return &Agent{
		deps:    deps,
		scanner: reposcanner.New(deps),
	}
}

// Run executes the planned workers in parallel, then aggregates findings.
func (a *Agent) Run(ctx context.Context, req RunRequest) (*domain.AuditState, error) {
	plan, err := supervisor.Plan(ctx, supervisor.Request{
		Scope:          req.Scope,
		Repos:          req.Repos,
		TargetRepoName: req.TargetRepoName,
	})
	if err != nil {
		return nil, err
	}

	state := &domain.AuditState{
		Repos:      plan.Repos,
		Scope:      plan.Scope,
		AuditRunID: plan.AuditRunID,
		DoraScores: map[string]domain.DoraScore{},
		RepoScores: map[string]domain.RepoScore{},
		Status:     "COMPLETE",
	}

	var (
		mu       sync.Mutex
		findings []domain.Finding
		metrics  = make(map[string]aggregator.RepoMetrics)
		errs     []domain.AuditError
	)

	workerSet := toSet(plan.Workers)
	var wg sync.WaitGroup

	runWorker := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				errs = append(errs, domain.AuditError{Node: name, Message: err.Error()})
				mu.Unlock()
			}
		}()
	}

	if workerSet["repo_scanner"] || workerSet["cicd_auditor"] || workerSet["coverage_analyzer"] {
		for _, repo := range plan.Repos {
			repo := repo

			if workerSet["repo_scanner"] {
				runWorker("repo_scanner", func() error {
					res, err := a.scanner.Scan(ctx, repo)
					if err != nil {
						return fmt.Errorf("%s: %w", repo.Name, err)
					}
					mu.Lock()
					findings = append(findings, res.Findings...)
					cur := metrics[repo.Name]
					cur.StandardizationScore = res.StandardizationScore
					metrics[repo.Name] = cur
					mu.Unlock()
					return nil
				})
			}

			if workerSet["cicd_auditor"] {
				runWorker("cicd_auditor", func() error {
					yaml, err := a.deps.FetchPipelineYAML(ctx, repo.Owner, repo.Name)
					if err != nil {
						return fmt.Errorf("%s: %w", repo.Name, err)
					}
					if yaml == "" {
						yaml = "jobs: {}" // missing pipeline still scoreable as immature
					}
					res, err := cicd.ScorePipeline(ctx, repo.Name, yaml)
					if err != nil {
						return fmt.Errorf("%s: %w", repo.Name, err)
					}
					mu.Lock()
					findings = append(findings, res.Findings...)
					cur := metrics[repo.Name]
					cur.CICDScore = res.TotalScore
					metrics[repo.Name] = cur
					mu.Unlock()
					return nil
				})
			}

			if workerSet["coverage_analyzer"] {
				runWorker("coverage_analyzer", func() error {
					in, err := a.deps.FetchCoverage(ctx, repo.Name)
					if err != nil {
						return fmt.Errorf("%s: %w", repo.Name, err)
					}
					res, err := coverage.Analyze(ctx, coverage.Report{
						RepoName:       repo.Name,
						LinePct:        in.LinePct,
						BranchPct:      in.BranchPct,
						CommitsPerWeek: in.CommitsPerWeek,
					})
					if err != nil {
						return fmt.Errorf("%s: %w", repo.Name, err)
					}
					mu.Lock()
					findings = append(findings, res.Findings...)
					cur := metrics[repo.Name]
					cur.CoverageLinePct = res.LinePct
					cur.CoverageRisk = res.ChangeRiskScore
					metrics[repo.Name] = cur
					mu.Unlock()
					return nil
				})
			}
		}
	}

	if workerSet["cloud_finops"] {
		runWorker("cloud_finops", func() error {
			inv, err := a.deps.FetchCloudInventory(ctx)
			if err != nil {
				return err
			}
			res, err := finops.Analyze(ctx, inv)
			if err != nil {
				return err
			}
			mu.Lock()
			findings = append(findings, res.Findings...)
			mu.Unlock()
			return nil
		})
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("audit cancelled: %w", err)
	}

	agg, err := aggregator.Aggregate(ctx, aggregator.Input{
		Findings:    findings,
		RepoMetrics: metrics,
	})
	if err != nil {
		return nil, err
	}

	for i := range agg.Findings {
		agg.Findings[i].AuditRunID = plan.AuditRunID
	}

	state.Findings = agg.Findings
	state.DoraScores = agg.DoraScores
	state.RepoScores = agg.RepoScores
	state.Errors = errs
	if len(errs) > 0 {
		state.Status = "PARTIAL_AUDIT"
	}
	return state, nil
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}
