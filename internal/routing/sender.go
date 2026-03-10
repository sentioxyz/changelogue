package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
)

// Notification is the payload sent through all notification channels.
type Notification struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Version     string `json:"version"`
	ProjectName string `json:"project_name,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Repository  string `json:"repository,omitempty"`
	ReleaseURL  string `json:"release_url,omitempty"`  // internal Changelogue link
	SourceURL   string `json:"source_url,omitempty"`   // upstream provider link
	TodoID      string `json:"todo_id,omitempty"`      // for constructing acknowledge/resolve URLs
	PublicURL   string `json:"-"`                      // base URL for action links (not serialized)
}

// Sender is the interface that all notification channel implementations must satisfy.
type Sender interface {
	Send(ctx context.Context, ch *models.NotificationChannel, msg Notification) error
}

// rawBodyFields contains the human-readable fields extracted from raw release JSON.
type rawBodyFields struct {
	Changelog  string
	ReleaseURL string
}

// parseRawBody extracts known fields from a raw JSON release body.
func parseRawBody(body string) (rawBodyFields, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return rawBodyFields{}, false
	}

	fields := rawBodyFields{
		Changelog:  stringField(raw, "changelog"),
		ReleaseURL: stringField(raw, "release_url"),
	}

	if fields.Changelog == "" && fields.ReleaseURL == "" {
		return fields, false
	}
	return fields, true
}

func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// ProviderURL builds the upstream release page URL for a given provider.
func ProviderURL(provider, repository, version string) string {
	switch strings.ToLower(provider) {
	case "github":
		return fmt.Sprintf("https://github.com/%s/releases/tag/%s", repository, version)
	case "dockerhub":
		return fmt.Sprintf("https://hub.docker.com/r/%s/tags?name=%s", repository, url.QueryEscape(version))
	case "ecr-public":
		return fmt.Sprintf("https://gallery.ecr.aws/%s", repository)
	case "gitlab":
		return fmt.Sprintf("https://gitlab.com/%s/-/releases/%s", repository, version)
	default:
		return ""
	}
}

// ProviderLabel returns a human-readable label for a provider.
func ProviderLabel(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "GitHub"
	case "dockerhub":
		return "Docker Hub"
	case "ecr-public":
		return "ECR Public"
	case "gitlab":
		return "GitLab"
	default:
		return provider
	}
}

// Regex patterns for markdown-to-ASCII conversion.
var (
	// GitHub PR/issue URLs → #NNN ( url )
	reGitHubPR = regexp.MustCompile(`https?://github\.com/([^/]+/[^/]+)/(pull|issues)/(\d+)`)
	// Markdown links [text](url) → text ( url )
	reMdLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	// Headings ## Text → dashed ASCII box
	reHeading = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	// Bold **text** and __text__
	reBoldStar = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldUnderscore = regexp.MustCompile(`__(.+?)__`)
	// Inline code `text`
	reInlineCode = regexp.MustCompile("`([^`]+)`")
	// HTML tags
	reHTML = regexp.MustCompile(`<[^>]+>`)
	// Images ![alt](url)
	reImage = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
)

// markdownToASCII converts GitHub-flavored Markdown into clean ASCII text
// suitable for display inside Slack/Discord code blocks.
func markdownToASCII(md string) string {
	s := md

	// Strip images first (before link processing)
	s = reImage.ReplaceAllString(s, "$1")

	// Convert GitHub PR/issue URLs to #NNN ( url )
	s = reGitHubPR.ReplaceAllString(s, "#$3 ( $0 )")

	// Convert markdown links [text](url) → text ( url )
	s = reMdLink.ReplaceAllString(s, "$1 ( $2 )")

	// Convert headings to ASCII art with dashes
	s = reHeading.ReplaceAllStringFunc(s, func(match string) string {
		parts := reHeading.FindStringSubmatch(match)
		title := strings.TrimSpace(parts[1])
		dashes := strings.Repeat("-", len(title))
		return dashes + "\n" + title + "\n" + dashes
	})

	// Strip bold markers
	s = reBoldStar.ReplaceAllString(s, "$1")
	s = reBoldUnderscore.ReplaceAllString(s, "$1")

	// Strip inline code backticks
	s = reInlineCode.ReplaceAllString(s, "$1")

	// Strip HTML tags
	s = reHTML.ReplaceAllString(s, "")

	// Collapse 3+ consecutive blank lines into 2
	reBlankLines := regexp.MustCompile(`\n{3,}`)
	s = reBlankLines.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}
