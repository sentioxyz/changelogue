package gate

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// NormalizeVersion applies a version mapping (regex + template) to a raw version
// string. If no mapping is provided or the regex is invalid, it falls back to
// stripping the "v"/"V" prefix and lowercasing.
func NormalizeVersion(raw string, mapping *models.VersionMapping) string {
	if mapping != nil && mapping.Pattern != "" {
		re, err := regexp.Compile(mapping.Pattern)
		if err == nil {
			matches := re.FindStringSubmatch(raw)
			if len(matches) > 1 {
				// Apply template with capture group substitution.
				result := mapping.Template
				for i := 1; i < len(matches); i++ {
					placeholder := fmt.Sprintf("$%d", i)
					result = strings.ReplaceAll(result, placeholder, matches[i])
				}
				if result != "" {
					return result
				}
			}
		}
	}
	// Default: strip v/V prefix, lowercase.
	v := strings.TrimPrefix(raw, "v")
	v = strings.TrimPrefix(v, "V")
	return strings.ToLower(v)
}

// NormalizeVersionForSource looks up the mapping for a source and normalizes the version.
func NormalizeVersionForSource(raw string, sourceID string, mappings map[string]models.VersionMapping) string {
	if m, ok := mappings[sourceID]; ok {
		return NormalizeVersion(raw, &m)
	}
	return NormalizeVersion(raw, nil)
}
