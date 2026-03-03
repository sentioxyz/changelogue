package models

import (
	"encoding/json"
	"testing"
)

func TestSemanticReportJSON(t *testing.T) {
	report := SemanticReport{
		Subject:          "Ready to Deploy: Geth v1.10.15 (Critical Update)",
		Urgency:          "Critical",
		UrgencyReason:    "Hard Fork detected in Discord #announcements",
		StatusChecks:     []string{"Docker Image Verified", "Binaries Available"},
		ChangelogSummary: "Fixes sync bug in block 14,000,000",
		Availability:     "GA",
		Adoption:         "12% of network updated",
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
	if decoded.Urgency != report.Urgency {
		t.Errorf("urgency: got %q, want %q", decoded.Urgency, report.Urgency)
	}
	if decoded.UrgencyReason != report.UrgencyReason {
		t.Errorf("urgency_reason: got %q, want %q", decoded.UrgencyReason, report.UrgencyReason)
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

func TestSemanticReportJSON_BackwardCompat(t *testing.T) {
	// Old-format JSON stored in database with risk_level/risk_reason
	oldJSON := `{
		"subject": "Ready to Deploy: Geth v1.10.15",
		"risk_level": "CRITICAL",
		"risk_reason": "Hard Fork detected",
		"status_checks": ["Docker Image Verified"],
		"changelog_summary": "Fixes sync bug",
		"availability": "GA",
		"adoption": "12% updated",
		"urgency": "Critical",
		"recommendation": "Wait for 25% adoption."
	}`

	var report SemanticReport
	if err := json.Unmarshal([]byte(oldJSON), &report); err != nil {
		t.Fatalf("unmarshal old format: %v", err)
	}
	if report.RiskLevel != "CRITICAL" {
		t.Errorf("risk_level: got %q, want CRITICAL", report.RiskLevel)
	}
	if report.RiskReason != "Hard Fork detected" {
		t.Errorf("risk_reason: got %q", report.RiskReason)
	}
}
