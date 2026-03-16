package auth

import "testing"

func TestAllowlistParseAndCheck(t *testing.T) {
	al := NewAllowlist("alice,bob", "myorg,otherog")

	if !al.IsUserAllowed("alice", nil) {
		t.Fatal("alice should be allowed by username")
	}
	if !al.IsUserAllowed("charlie", []string{"myorg"}) {
		t.Fatal("charlie should be allowed by org membership")
	}
	if al.IsUserAllowed("charlie", []string{"randorg"}) {
		t.Fatal("charlie with no matching org should be denied")
	}
	if al.IsUserAllowed("charlie", nil) {
		t.Fatal("charlie with no orgs should be denied")
	}
}

func TestAllowlistEmpty(t *testing.T) {
	al := NewAllowlist("", "")
	if al.HasEntries() {
		t.Fatal("empty allowlist should report no entries")
	}
}

func TestAllowlistCaseInsensitive(t *testing.T) {
	al := NewAllowlist("Alice", "MyOrg")
	if !al.IsUserAllowed("alice", nil) {
		t.Fatal("username check should be case-insensitive")
	}
	if !al.IsUserAllowed("bob", []string{"myorg"}) {
		t.Fatal("org check should be case-insensitive")
	}
}
