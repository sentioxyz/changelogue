package routing

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sentioxyz/releaseguard/internal/models"
)

// CheckAgentRules evaluates the agent rules against the new and previous version
// strings. Returns true if any rule matches and an agent run should be triggered.
func CheckAgentRules(rules *models.AgentRules, newVersion, previousVersion string) bool {
	if rules == nil {
		return false
	}

	newMajor, newMinor, _ := parseVersion(newVersion)
	oldMajor, oldMinor, _ := parseVersion(previousVersion)

	if rules.OnMajorRelease && newMajor > oldMajor {
		return true
	}
	if rules.OnMinorRelease && (newMajor > oldMajor || newMinor > oldMinor) {
		return true
	}
	if rules.OnSecurityPatch && isSecurityPatch(newVersion) {
		return true
	}
	if rules.VersionPattern != "" {
		if matched, _ := regexp.MatchString(rules.VersionPattern, newVersion); matched {
			return true
		}
	}
	return false
}

// parseVersion extracts major, minor, patch integers from a version string.
// It tolerates a leading "v" prefix and pre-release suffixes (e.g. "-security").
func parseVersion(v string) (major, minor, patch int) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		// Strip pre-release suffix (e.g. "4-security" -> "4")
		patchStr := strings.SplitN(parts[2], "-", 2)[0]
		patch, _ = strconv.Atoi(patchStr)
	}
	return
}

// isSecurityPatch returns true if the version string contains a known security
// keyword in its pre-release or metadata suffix.
func isSecurityPatch(version string) bool {
	lower := strings.ToLower(version)
	keywords := []string{"security", "cve", "vuln"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
