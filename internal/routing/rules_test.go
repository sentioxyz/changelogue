package routing

import (
	"testing"

	"github.com/sentioxyz/releaseguard/internal/models"
)

func TestCheckAgentRules_MajorRelease(t *testing.T) {
	rules := &models.AgentRules{OnMajorRelease: true}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if !triggered {
		t.Fatal("expected agent to be triggered for major version bump")
	}
}

func TestCheckAgentRules_MinorRelease(t *testing.T) {
	rules := &models.AgentRules{OnMinorRelease: true}
	triggered := CheckAgentRules(rules, "v1.6.0", "v1.5.3")
	if !triggered {
		t.Fatal("expected agent to be triggered for minor version bump")
	}
}

func TestCheckAgentRules_NoMatch(t *testing.T) {
	rules := &models.AgentRules{OnMajorRelease: true}
	triggered := CheckAgentRules(rules, "v1.5.4", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered for patch bump")
	}
}

func TestCheckAgentRules_SecurityPatch(t *testing.T) {
	rules := &models.AgentRules{OnSecurityPatch: true}
	triggered := CheckAgentRules(rules, "v1.5.4-security", "v1.5.3")
	if !triggered {
		t.Fatal("expected agent to be triggered for security patch")
	}
}

func TestCheckAgentRules_VersionPattern(t *testing.T) {
	rules := &models.AgentRules{VersionPattern: `^v2\.`}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if !triggered {
		t.Fatal("expected agent to be triggered for version pattern match")
	}
}

func TestCheckAgentRules_NilRules(t *testing.T) {
	triggered := CheckAgentRules(nil, "v2.0.0", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered for nil rules")
	}
}

func TestCheckAgentRules_EmptyPreviousVersion(t *testing.T) {
	rules := &models.AgentRules{OnMajorRelease: true}
	triggered := CheckAgentRules(rules, "v1.0.0", "")
	if !triggered {
		t.Fatal("expected agent to be triggered for first release with major rule")
	}
}

func TestCheckAgentRules_MinorReleaseAcrossMajor(t *testing.T) {
	rules := &models.AgentRules{OnMinorRelease: true}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if !triggered {
		t.Fatal("expected agent to be triggered for minor rule on major bump (major includes minor)")
	}
}

func TestCheckAgentRules_PatchOnly_NoTrigger(t *testing.T) {
	rules := &models.AgentRules{OnMinorRelease: true}
	triggered := CheckAgentRules(rules, "v1.5.4", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered for patch bump with minor rule")
	}
}

func TestCheckAgentRules_SecurityKeywords(t *testing.T) {
	rules := &models.AgentRules{OnSecurityPatch: true}

	tests := []struct {
		version string
		want    bool
	}{
		{"v1.0.1-security", true},
		{"v1.0.1-cve-2025-1234", true},
		{"v1.0.1-vuln-fix", true},
		{"v1.0.1-SECURITY", true},
		{"v1.0.1-patch", false},
		{"v1.0.1", false},
	}

	for _, tt := range tests {
		got := CheckAgentRules(rules, tt.version, "v1.0.0")
		if got != tt.want {
			t.Errorf("CheckAgentRules(security, %q): got %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestCheckAgentRules_VersionPatternNoMatch(t *testing.T) {
	rules := &models.AgentRules{VersionPattern: `^v3\.`}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered when version pattern doesn't match")
	}
}

func TestCheckAgentRules_InvalidPattern(t *testing.T) {
	rules := &models.AgentRules{VersionPattern: `[invalid`}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered for invalid regex pattern")
	}
}

func TestCheckAgentRules_AllRulesFalse(t *testing.T) {
	rules := &models.AgentRules{}
	triggered := CheckAgentRules(rules, "v2.0.0", "v1.5.3")
	if triggered {
		t.Fatal("expected agent not to be triggered when all rules are false")
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input                      string
		wantMajor, wantMinor, wantPatch int
	}{
		{"v1.5.3", 1, 5, 3},
		{"v2.0.0", 2, 0, 0},
		{"1.2.3", 1, 2, 3},
		{"v1.5.4-security", 1, 5, 4},
		{"v0.0.0", 0, 0, 0},
		{"", 0, 0, 0},
		{"v1", 1, 0, 0},
		{"v1.2", 1, 2, 0},
	}

	for _, tt := range tests {
		major, minor, patch := parseVersion(tt.input)
		if major != tt.wantMajor || minor != tt.wantMinor || patch != tt.wantPatch {
			t.Errorf("parseVersion(%q) = (%d, %d, %d), want (%d, %d, %d)",
				tt.input, major, minor, patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
		}
	}
}
