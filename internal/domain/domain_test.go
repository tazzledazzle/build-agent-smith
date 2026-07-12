package domain_test

import (
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

func TestPriorityScore(t *testing.T) {
	tests := []struct {
		name             string
		costImpactUSD    float64
		severity         domain.Severity
		remediationHours float64
		want             float64
	}{
		{
			name:             "critical high cost low effort",
			costImpactUSD:    1000,
			severity:         domain.SeverityCritical,
			remediationHours: 2,
			want:             2000, // 1000 * 4 / 2
		},
		{
			name:             "low severity high effort",
			costImpactUSD:    100,
			severity:         domain.SeverityLow,
			remediationHours: 10,
			want:             10, // 100 * 1 / 10
		},
		{
			name:             "zero effort uses minimum of one hour",
			costImpactUSD:    500,
			severity:         domain.SeverityHigh,
			remediationHours: 0,
			want:             1500, // 500 * 3 / 1
		},
		{
			name:             "medium severity",
			costImpactUSD:    200,
			severity:         domain.SeverityMedium,
			remediationHours: 4,
			want:             100, // 200 * 2 / 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.PriorityScore(tt.costImpactUSD, tt.severity, tt.remediationHours)
			if got != tt.want {
				t.Errorf("PriorityScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		severity domain.Severity
		want     float64
	}{
		{domain.SeverityCritical, 4},
		{domain.SeverityHigh, 3},
		{domain.SeverityMedium, 2},
		{domain.SeverityLow, 1},
		{domain.Severity("unknown"), 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := domain.SeverityWeight(tt.severity)
			if got != tt.want {
				t.Errorf("SeverityWeight(%q) = %v, want %v", tt.severity, got, tt.want)
			}
		})
	}
}
