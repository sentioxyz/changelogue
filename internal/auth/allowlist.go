package auth

import "strings"

// Allowlist restricts login to specific GitHub users and/or org members.
type Allowlist struct {
	users map[string]bool
	orgs  map[string]bool
}

// NewAllowlist parses comma-separated username and org lists.
func NewAllowlist(users, orgs string) *Allowlist {
	al := &Allowlist{
		users: make(map[string]bool),
		orgs:  make(map[string]bool),
	}
	for _, u := range strings.Split(users, ",") {
		u = strings.TrimSpace(strings.ToLower(u))
		if u != "" {
			al.users[u] = true
		}
	}
	for _, o := range strings.Split(orgs, ",") {
		o = strings.TrimSpace(strings.ToLower(o))
		if o != "" {
			al.orgs[o] = true
		}
	}
	return al
}

// HasEntries returns true if at least one user or org is configured.
func (a *Allowlist) HasEntries() bool {
	return len(a.users) > 0 || len(a.orgs) > 0
}

// IsUserAllowed checks if a GitHub login or any of their org memberships match.
func (a *Allowlist) IsUserAllowed(login string, orgLogins []string) bool {
	if a.users[strings.ToLower(login)] {
		return true
	}
	for _, o := range orgLogins {
		if a.orgs[strings.ToLower(o)] {
			return true
		}
	}
	return false
}
