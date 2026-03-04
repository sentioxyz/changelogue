package agent

import (
	"context"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/sentioxyz/changelogue/internal/models"
)

// AgentDataStore defines the data access methods that agent tools use to
// research releases and context sources for a given project.
type AgentDataStore interface {
	ListReleasesByProject(ctx context.Context, projectID string, page, perPage int, includeExcluded bool) ([]models.Release, int, error)
	GetRelease(ctx context.Context, id string) (*models.Release, error)
	ListContextSources(ctx context.Context, projectID string, page, perPage int) ([]models.ContextSource, int, error)
	ListSourcesByProject(ctx context.Context, projectID string, page, perPage int) ([]models.Source, int, error)
}

// --- Tool input/output types ---

// GetReleasesInput is the input for the get_releases tool.
type GetReleasesInput struct {
	Page    int `json:"page,omitempty"`
	PerPage int `json:"per_page,omitempty"`
}

// ReleaseItem is a single release in the tool output.
type ReleaseItem struct {
	ID         string `json:"id"`
	SourceID   string `json:"source_id"`
	Version    string `json:"version"`
	RawData    any    `json:"raw_data,omitempty"`
	ReleasedAt string `json:"released_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// GetReleasesOutput is the output for the get_releases tool.
type GetReleasesOutput struct {
	Releases []ReleaseItem `json:"releases"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PerPage  int           `json:"per_page"`
}

// GetReleaseDetailInput is the input for the get_release_detail tool.
type GetReleaseDetailInput struct {
	ReleaseID string `json:"release_id"`
}

// GetReleaseDetailOutput is the output for the get_release_detail tool.
type GetReleaseDetailOutput struct {
	ID         string `json:"id"`
	SourceID   string `json:"source_id"`
	Version    string `json:"version"`
	RawData    any    `json:"raw_data,omitempty"`
	ReleasedAt string `json:"released_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ListContextSourcesInput is the input for the list_context_sources tool.
type ListContextSourcesInput struct {
	Page    int `json:"page,omitempty"`
	PerPage int `json:"per_page,omitempty"`
}

// ContextSourceItem is a single context source in the tool output.
type ContextSourceItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Config any    `json:"config"`
}

// ListContextSourcesOutput is the output for the list_context_sources tool.
type ListContextSourcesOutput struct {
	Sources []ContextSourceItem `json:"sources"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"per_page"`
}

// toolFactory holds the store reference and project ID scoped to a single
// agent run. Tools created from this factory are bound to a specific project.
type toolFactory struct {
	store     AgentDataStore
	projectID string
}

// NewTools creates the ADK function tools for agent research, scoped to a
// specific project. The returned tools use the provided store to query
// releases and context sources belonging to projectID.
func NewTools(store AgentDataStore, projectID string) ([]tool.Tool, error) {
	f := &toolFactory{store: store, projectID: projectID}

	getReleases, err := functiontool.New(functiontool.Config{
		Name:        "get_releases",
		Description: "Fetch recent releases for the current project. Returns a paginated list of releases with version, raw data, and timestamps. Use this to understand what new versions have been published.",
	}, f.getReleases)
	if err != nil {
		return nil, fmt.Errorf("create get_releases tool: %w", err)
	}

	getReleaseDetail, err := functiontool.New(functiontool.Config{
		Name:        "get_release_detail",
		Description: "Get detailed information for a specific release by its ID. Returns the full raw data payload from the upstream provider. Use this to inspect changelogs, commit lists, or other release metadata.",
	}, f.getReleaseDetail)
	if err != nil {
		return nil, fmt.Errorf("create get_release_detail tool: %w", err)
	}

	listContextSources, err := functiontool.New(functiontool.Config{
		Name:        "list_context_sources",
		Description: "List the context sources configured for this project. Context sources are references to runbooks, documentation, monitoring dashboards, and other background materials that help you produce richer analysis.",
	}, f.listContextSources)
	if err != nil {
		return nil, fmt.Errorf("create list_context_sources tool: %w", err)
	}

	return []tool.Tool{getReleases, getReleaseDetail, listContextSources}, nil
}

// getReleases is the handler for the get_releases tool.
func (f *toolFactory) getReleases(ctx tool.Context, input GetReleasesInput) (GetReleasesOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	perPage := input.PerPage
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}

	releases, total, err := f.store.ListReleasesByProject(ctx, f.projectID, page, perPage, false)
	if err != nil {
		return GetReleasesOutput{}, fmt.Errorf("list releases: %w", err)
	}

	items := make([]ReleaseItem, 0, len(releases))
	for _, r := range releases {
		item := ReleaseItem{
			ID:        r.ID,
			SourceID:  r.SourceID,
			Version:   r.Version,
			RawData:   r.RawData,
			CreatedAt: r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if r.ReleasedAt != nil {
			item.ReleasedAt = r.ReleasedAt.Format("2006-01-02T15:04:05Z")
		}
		items = append(items, item)
	}

	return GetReleasesOutput{
		Releases: items,
		Total:    total,
		Page:     page,
		PerPage:  perPage,
	}, nil
}

// getReleaseDetail is the handler for the get_release_detail tool.
func (f *toolFactory) getReleaseDetail(ctx tool.Context, input GetReleaseDetailInput) (GetReleaseDetailOutput, error) {
	if input.ReleaseID == "" {
		return GetReleaseDetailOutput{}, fmt.Errorf("release_id is required")
	}

	release, err := f.store.GetRelease(ctx, input.ReleaseID)
	if err != nil {
		return GetReleaseDetailOutput{}, fmt.Errorf("get release: %w", err)
	}

	out := GetReleaseDetailOutput{
		ID:        release.ID,
		SourceID:  release.SourceID,
		Version:   release.Version,
		RawData:   release.RawData,
		CreatedAt: release.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if release.ReleasedAt != nil {
		out.ReleasedAt = release.ReleasedAt.Format("2006-01-02T15:04:05Z")
	}

	return out, nil
}

// listContextSources is the handler for the list_context_sources tool.
func (f *toolFactory) listContextSources(ctx tool.Context, input ListContextSourcesInput) (ListContextSourcesOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	perPage := input.PerPage
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}

	sources, total, err := f.store.ListContextSources(ctx, f.projectID, page, perPage)
	if err != nil {
		return ListContextSourcesOutput{}, fmt.Errorf("list context sources: %w", err)
	}

	items := make([]ContextSourceItem, 0, len(sources))
	for _, s := range sources {
		items = append(items, ContextSourceItem{
			ID:     s.ID,
			Type:   s.Type,
			Name:   s.Name,
			Config: s.Config,
		})
	}

	return ListContextSourcesOutput{
		Sources: items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}
