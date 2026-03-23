package cli

import (
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// ListReleases fetches a paginated list of releases.
// Path-based routing: if sourceID is set, uses /sources/{id}/releases;
// if projectID is set, uses /projects/{id}/releases; otherwise /releases.
func ListReleases(c *Client, sourceID, projectID string, includeExcluded bool, page, perPage int) ([]models.Release, Meta, error) {
	var path string
	switch {
	case sourceID != "":
		path = fmt.Sprintf("/api/v1/sources/%s/releases", sourceID)
	case projectID != "":
		path = fmt.Sprintf("/api/v1/projects/%s/releases", projectID)
	default:
		path = "/api/v1/releases"
	}
	path += fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	if includeExcluded {
		path += "&include_excluded=true"
	}
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Release]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// GetRelease fetches a single release by ID.
func GetRelease(c *Client, id string) (*models.Release, error) {
	resp, err := c.Get("/api/v1/releases/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Release]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// --- Cobra commands ---

// NewReleasesCmd returns the "releases" command group.
// clientFn is called at execution time to build the client from resolved flags.
func NewReleasesCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Browse releases",
		Long:  "List and view releases.\n\nExamples:\n  clog releases list\n  clog releases list --project <id>\n  clog releases list --source <id>\n  clog releases get <id>",
	}

	var page, perPage int
	var sourceID, projectID string
	var includeExcluded bool

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List releases",
		Example: "  clog releases list\n  clog releases list --project abc-123\n  clog releases list --source src-123 --include-excluded",
		RunE: func(cmd *cobra.Command, args []string) error {
			releases, meta, err := ListReleases(clientFn(), sourceID, projectID, includeExcluded, page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": releases, "meta": meta})
				return nil
			}
			rows := make([][]string, len(releases))
			for i, r := range releases {
				releasedAt := ""
				if r.ReleasedAt != nil {
					releasedAt = FormatTime(r.ReleasedAt.Format("2006-01-02T15:04:05"))
				}
				rows[i] = []string{r.ID, r.Version, r.Provider, r.Repository, releasedAt, r.SemanticReleaseStatus}
			}
			RenderTable([]string{"ID", "VERSION", "PROVIDER", "REPOSITORY", "RELEASED", "SEMANTIC STATUS"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().StringVar(&sourceID, "source", "", "Filter by source ID")
	listCmd.Flags().StringVar(&projectID, "project", "", "Filter by project ID")
	listCmd.Flags().BoolVar(&includeExcluded, "include-excluded", false, "Include releases filtered out by version patterns")
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Get release details",
		Args:    cobra.ExactArgs(1),
		Example: "  clog releases get rel-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			rel, err := GetRelease(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(rel)
				return nil
			}
			releasedAt := ""
			if rel.ReleasedAt != nil {
				releasedAt = FormatTime(rel.ReleasedAt.Format("2006-01-02T15:04:05"))
			}
			rows := [][]string{{rel.ID, rel.Version, rel.Provider, rel.Repository, releasedAt}}
			RenderTable([]string{"ID", "VERSION", "PROVIDER", "REPOSITORY", "RELEASED"}, rows)
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd)
	return cmd
}
