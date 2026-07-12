// Package domain defines shared types for the platform maturity audit agent.
package domain

import "time"

// Scope controls which worker nodes the supervisor invokes.
type Scope string

const (
	ScopeFull        Scope = "full"
	ScopeIncremental Scope = "incremental"
	ScopeFinOpsOnly  Scope = "finops_only"
)

// FindingType categorizes audit findings.
type FindingType string

const (
	FindingTypeCICDGap         FindingType = "cicd_gap"
	FindingTypeCoverageRisk    FindingType = "coverage_risk"
	FindingTypeCloudWaste      FindingType = "cloud_waste"
	FindingTypeStandardization FindingType = "standardization"
)

// Severity ranks finding urgency.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// FindingStatus tracks remediation lifecycle.
type FindingStatus string

const (
	FindingStatusOpen         FindingStatus = "open"
	FindingStatusAcknowledged FindingStatus = "acknowledged"
	FindingStatusResolved     FindingStatus = "resolved"
)

// RepoConfig describes a repository in the audit manifest.
type RepoConfig struct {
	Name          string `json:"name"`
	Owner         string `json:"owner"`
	Provider      string `json:"provider"` // github | gitlab
	DefaultBranch string `json:"default_branch"`
}

// Finding is a single actionable audit result.
type Finding struct {
	ID                     string
	AuditRunID             string
	RepoName               string
	FindingType            FindingType
	Severity               Severity
	Title                  string
	Description            string
	EstimatedCostUSD       float64
	RemediationEffortHours float64
	PriorityScore          float64
	ResourceID             string
	OwnerTag               string
	Status                 FindingStatus
	CreatedAt              time.Time
}

// DoraScore holds DORA-aligned maturity proxies for a repo.
type DoraScore struct {
	DeploymentFrequency float64
	LeadTimeDays        float64
	ChangeFailureRate   float64
	MTTRHours           float64
	Composite           float64
}

// RepoScore aggregates per-repo metrics for an audit run.
type RepoScore struct {
	AuditRunID           string
	RepoName             string
	StandardizationScore int
	CICDMaturityScore    int
	CoverageLinePct      float64
	CoverageRiskScore    float64
	DoraCompositeScore   float64
}

// AuditError captures a non-fatal worker failure.
type AuditError struct {
	Node    string
	Repo    string
	Message string
}

// AuditState is the shared agent state passed between nodes.
type AuditState struct {
	Repos      []RepoConfig
	Scope      Scope
	Findings   []Finding
	DoraScores map[string]DoraScore
	RepoScores map[string]RepoScore
	AuditRunID string
	Errors     []AuditError
	Status     string // COMPLETE | PARTIAL_AUDIT
}

// AuditRun is the persisted audit_runs row.
type AuditRun struct {
	ID                     string
	TriggeredAt            time.Time
	Scope                  Scope
	RepoCount              int
	TotalFindings          int
	TotalEstimatedWasteUSD float64
	CompletedAt            time.Time
	Status                 string
}

// SeverityWeight returns the numeric weight used in priority ranking.
func SeverityWeight(s Severity) float64 {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	default:
		return 1
	}
}

// PriorityScore computes cost_impact * severity_weight / remediation_hours.
// Zero or negative effort is treated as 1 hour to avoid division by zero.
func PriorityScore(costImpactUSD float64, severity Severity, remediationHours float64) float64 {
	effort := remediationHours
	if effort <= 0 {
		effort = 1
	}
	return costImpactUSD * SeverityWeight(severity) / effort
}
