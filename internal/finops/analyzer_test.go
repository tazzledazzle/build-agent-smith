package finops_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/finops"
)

func TestAnalyze_IdleEC2(t *testing.T) {
	inventory := finops.Inventory{
		EC2Instances: []finops.EC2Instance{
			{
				ID:                  "i-idle",
				AvgCPU14d:           3.2,
				EstimatedMonthlyUSD: 120,
				Tags:                map[string]string{"team": "platform", "service": "batch"},
			},
		},
	}
	result, err := finops.Analyze(context.Background(), inventory)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(result.Findings))
	}
	f := result.Findings[0]
	if f.FindingType != domain.FindingTypeCloudWaste {
		t.Errorf("FindingType = %q", f.FindingType)
	}
	if f.ResourceID != "i-idle" {
		t.Errorf("ResourceID = %q", f.ResourceID)
	}
	if f.EstimatedCostUSD != 120 {
		t.Errorf("EstimatedCostUSD = %v, want 120", f.EstimatedCostUSD)
	}
	if !contains(result.Flags, "IDLE") {
		t.Errorf("Flags = %v, want IDLE", result.Flags)
	}
}

func TestAnalyze_OverProvisionedRDS(t *testing.T) {
	inventory := finops.Inventory{
		RDSInstances: []finops.RDSInstance{
			{
				ID:                  "db-fat",
				AvgCPUUtilization:   12,
				EstimatedMonthlyUSD: 400,
				Tags:                map[string]string{"team": "data", "service": "analytics"},
			},
		},
	}
	result, err := finops.Analyze(context.Background(), inventory)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !contains(result.Flags, "OVER_PROVISIONED") {
		t.Errorf("Flags = %v, want OVER_PROVISIONED", result.Flags)
	}
}

func TestAnalyze_OrphanedEBS(t *testing.T) {
	inventory := finops.Inventory{
		EBSVolumes: []finops.EBSVolume{
			{ID: "vol-orphan", Attached: false, EstimatedMonthlyUSD: 40, Tags: map[string]string{"team": "infra", "service": "logs"}},
		},
	}
	result, err := finops.Analyze(context.Background(), inventory)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !contains(result.Flags, "ORPHANED") {
		t.Errorf("Flags = %v, want ORPHANED", result.Flags)
	}
}

func TestAnalyze_Untagged(t *testing.T) {
	inventory := finops.Inventory{
		EC2Instances: []finops.EC2Instance{
			{ID: "i-naked", AvgCPU14d: 50, EstimatedMonthlyUSD: 80, Tags: map[string]string{}},
		},
	}
	result, err := finops.Analyze(context.Background(), inventory)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !contains(result.Flags, "UNTAGGED") {
		t.Errorf("Flags = %v, want UNTAGGED", result.Flags)
	}
	found := false
	for _, f := range result.Findings {
		if f.ResourceID == "i-naked" && f.Severity == domain.SeverityMedium {
			found = true
		}
	}
	if !found {
		t.Error("expected medium severity untagged finding")
	}
}

func TestAnalyze_HealthyResourcesProduceNoFlags(t *testing.T) {
	inventory := finops.Inventory{
		EC2Instances: []finops.EC2Instance{
			{ID: "i-ok", AvgCPU14d: 45, EstimatedMonthlyUSD: 100, Tags: map[string]string{"team": "a", "service": "b"}},
		},
		RDSInstances: []finops.RDSInstance{
			{ID: "db-ok", AvgCPUUtilization: 55, EstimatedMonthlyUSD: 200, Tags: map[string]string{"team": "a", "service": "b"}},
		},
		EBSVolumes: []finops.EBSVolume{
			{ID: "vol-ok", Attached: true, EstimatedMonthlyUSD: 20, Tags: map[string]string{"team": "a", "service": "b"}},
		},
	}
	result, err := finops.Analyze(context.Background(), inventory)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings, got %+v", result.Findings)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
