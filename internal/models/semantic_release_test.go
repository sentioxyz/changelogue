package models

import (
	"encoding/json"
	"testing"
)

func TestSemanticReportJSON(t *testing.T) {
	report := SemanticReport{
		Subject:          "🚀 Ready to Deploy: Geth v1.10.15 (Critical Update)",
		RiskLevel:        "CRITICAL",
		RiskReason:       "Hard Fork detected in Discord #announcements",
		StatusChecks:     []string{"Docker Image Verified", "Binaries Available"},
		ChangelogSummary: "Fixes sync bug in block 14,000,000",
		Availability:     "GA",
		Adoption:         "12% of network updated",
		Urgency:          "Critical",
		Recommendation:   "Wait for 25% adoption unless urgent.",
		DownloadCommands: []string{"docker pull ethereum/client-go:v1.10.15"},
		DownloadLinks:    []string{"https://github.com/ethereum/go-ethereum/releases/tag/v1.10.15"},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SemanticReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Subject != report.Subject {
		t.Errorf("subject: got %q, want %q", decoded.Subject, report.Subject)
	}
	if decoded.RiskLevel != report.RiskLevel {
		t.Errorf("risk_level: got %q, want %q", decoded.RiskLevel, report.RiskLevel)
	}
	if len(decoded.StatusChecks) != 2 {
		t.Errorf("status_checks: got %d items, want 2", len(decoded.StatusChecks))
	}
	if len(decoded.DownloadCommands) != 1 {
		t.Errorf("download_commands: got %d items, want 1", len(decoded.DownloadCommands))
	}
	if len(decoded.DownloadLinks) != 1 {
		t.Errorf("download_links: got %d items, want 1", len(decoded.DownloadLinks))
	}
}
