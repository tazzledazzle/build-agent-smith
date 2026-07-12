// Package finops detects cloud waste from utilization and tagging signals.
package finops

import (
	"context"
	"fmt"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// EC2Instance is a snapshot of EC2 utilization for FinOps analysis.
type EC2Instance struct {
	ID                  string
	AvgCPU14d           float64
	EstimatedMonthlyUSD float64
	Tags                map[string]string
}

// RDSInstance is a snapshot of RDS utilization.
type RDSInstance struct {
	ID                  string
	AvgCPUUtilization   float64
	EstimatedMonthlyUSD float64
	Tags                map[string]string
}

// EBSVolume is a snapshot of EBS attachment state.
type EBSVolume struct {
	ID                  string
	Attached            bool
	EstimatedMonthlyUSD float64
	Tags                map[string]string
}

// Inventory is the cloud resource input for a FinOps scan.
type Inventory struct {
	EC2Instances []EC2Instance
	RDSInstances []RDSInstance
	EBSVolumes   []EBSVolume
}

// Result holds FinOps flags and findings.
type Result struct {
	Flags    []string
	Findings []domain.Finding
}

// Analyze flags idle, over-provisioned, orphaned, and untagged resources.
func Analyze(ctx context.Context, inv Inventory) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("finops analyze: %w", err)
	}

	result := &Result{}
	flagSet := map[string]struct{}{}

	addFlag := func(flag string) {
		if _, ok := flagSet[flag]; !ok {
			flagSet[flag] = struct{}{}
			result.Flags = append(result.Flags, flag)
		}
	}

	for _, inst := range inv.EC2Instances {
		if missingOwnerTags(inst.Tags) {
			addFlag("UNTAGGED")
			result.Findings = append(result.Findings, cloudFinding(
				inst.ID, "EC2", "UNTAGGED", domain.SeverityMedium,
				"Resource missing team or service tags", inst.EstimatedMonthlyUSD, 1, ownerTag(inst.Tags)))
		}
		if inst.AvgCPU14d < 5 {
			addFlag("IDLE")
			result.Findings = append(result.Findings, cloudFinding(
				inst.ID, "EC2", "IDLE", domain.SeverityHigh,
				fmt.Sprintf("EC2 CPU avg %.1f%% over 14 days (<5%%)", inst.AvgCPU14d),
				inst.EstimatedMonthlyUSD, 2, ownerTag(inst.Tags)))
		}
	}

	for _, db := range inv.RDSInstances {
		if missingOwnerTags(db.Tags) {
			addFlag("UNTAGGED")
			result.Findings = append(result.Findings, cloudFinding(
				db.ID, "RDS", "UNTAGGED", domain.SeverityMedium,
				"Resource missing team or service tags", db.EstimatedMonthlyUSD, 1, ownerTag(db.Tags)))
		}
		if db.AvgCPUUtilization < 20 {
			addFlag("OVER_PROVISIONED")
			result.Findings = append(result.Findings, cloudFinding(
				db.ID, "RDS", "OVER_PROVISIONED", domain.SeverityHigh,
				fmt.Sprintf("RDS avg utilization %.1f%% (<20%%)", db.AvgCPUUtilization),
				db.EstimatedMonthlyUSD*0.4, 4, ownerTag(db.Tags)))
		}
	}

	for _, vol := range inv.EBSVolumes {
		if missingOwnerTags(vol.Tags) {
			addFlag("UNTAGGED")
			result.Findings = append(result.Findings, cloudFinding(
				vol.ID, "EBS", "UNTAGGED", domain.SeverityMedium,
				"Resource missing team or service tags", vol.EstimatedMonthlyUSD, 1, ownerTag(vol.Tags)))
		}
		if !vol.Attached {
			addFlag("ORPHANED")
			result.Findings = append(result.Findings, cloudFinding(
				vol.ID, "EBS", "ORPHANED", domain.SeverityHigh,
				"EBS volume has no attachment", vol.EstimatedMonthlyUSD, 1, ownerTag(vol.Tags)))
		}
	}

	return result, nil
}

func missingOwnerTags(tags map[string]string) bool {
	_, hasTeam := tags["team"]
	_, hasService := tags["service"]
	return !hasTeam || !hasService
}

func ownerTag(tags map[string]string) string {
	if t, ok := tags["team"]; ok {
		return t
	}
	return ""
}

func cloudFinding(resourceID, resourceType, flag string, sev domain.Severity, desc string, cost, hours float64, owner string) domain.Finding {
	f := domain.Finding{
		FindingType:            domain.FindingTypeCloudWaste,
		Severity:               sev,
		Title:                  fmt.Sprintf("%s %s (%s)", flag, resourceType, resourceID),
		Description:            desc,
		EstimatedCostUSD:       cost,
		RemediationEffortHours: hours,
		ResourceID:             resourceID,
		OwnerTag:               owner,
		Status:                 domain.FindingStatusOpen,
		CreatedAt:              time.Now().UTC(),
	}
	f.PriorityScore = domain.PriorityScore(f.EstimatedCostUSD, f.Severity, f.RemediationEffortHours)
	return f
}
