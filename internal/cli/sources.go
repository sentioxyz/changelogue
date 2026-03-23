package cli

import (
	"fmt"
	"net/url"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// ListSources fetches a paginated list of sources for a project.
func ListSources(c *Client, projectID string, page, perPage int) ([]models.Source, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/sources?page=%d&per_page=%d", url.PathEscape(projectID), page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// GetSource fetches a single source by ID.
func GetSource(c *Client, id string) (*models.Source, error) {
	resp, err := c.Get("/api/v1/sources/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateSource creates a new source under the given project.
func CreateSource(c *Client, projectID string, body map[string]any) (*models.Source, error) {
	resp, err := c.Post("/api/v1/projects/"+url.PathEscape(projectID)+"/sources", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateSource updates an existing source with the given fields.
func UpdateSource(c *Client, id string, fields map[string]any) (*models.Source, error) {
	resp, err := c.Put("/api/v1/sources/"+url.PathEscape(id), fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Source]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteSource deletes a source by ID.
func DeleteSource(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/sources/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// --- Cobra commands ---

// NewSourcesCmd returns the "sources" command group.
// clientFn is called at execution time to build the client from resolved flags.
func NewSourcesCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage ingestion sources",
		Long:  "Add, list, update, and remove sources for a project.\n\nProviders: dockerhub, github, ecr, gitlab, pypi, npm\n\nExamples:\n  clog sources list --project <id>\n  clog sources create --project <id> --provider dockerhub --repository library/postgres",
	}

	var page, perPage int

	// --- list ---
	var listProjectID string
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List sources for a project",
		Example: "  clog sources list --project abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			sources, meta, err := ListSources(clientFn(), listProjectID, page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": sources, "meta": meta})
				return nil
			}
			rows := make([][]string, len(sources))
			for i, s := range sources {
				enabled := "yes"
				if !s.Enabled {
					enabled = "no"
				}
				rows[i] = []string{s.ID, s.Provider, s.Repository, enabled, fmt.Sprintf("%ds", s.PollIntervalSeconds)}
			}
			RenderTable([]string{"ID", "PROVIDER", "REPOSITORY", "ENABLED", "POLL INTERVAL"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().StringVar(&listProjectID, "project", "", "Project ID (required)")
	listCmd.MarkFlagRequired("project")
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Get source details",
		Args:    cobra.ExactArgs(1),
		Example: "  clog sources get src-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := GetSource(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(src)
				return nil
			}
			enabled := "yes"
			if !src.Enabled {
				enabled = "no"
			}
			rows := [][]string{{src.ID, src.Provider, src.Repository, enabled, fmt.Sprintf("%ds", src.PollIntervalSeconds)}}
			RenderTable([]string{"ID", "PROVIDER", "REPOSITORY", "ENABLED", "POLL INTERVAL"}, rows)
			return nil
		},
	}

	// --- create ---
	var createProjectID, createProvider, createRepo, createFilterInclude, createFilterExclude string
	var createPollInterval int
	var createExcludePrerelease bool
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Add a new source",
		Example: "  clog sources create --project abc-123 --provider dockerhub --repository library/postgres",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"provider":   createProvider,
				"repository": createRepo,
			}
			if createPollInterval > 0 {
				body["poll_interval_seconds"] = createPollInterval
			}
			if createFilterInclude != "" {
				body["version_filter_include"] = createFilterInclude
			}
			if createFilterExclude != "" {
				body["version_filter_exclude"] = createFilterExclude
			}
			if createExcludePrerelease {
				body["exclude_prereleases"] = true
			}
			src, err := CreateSource(clientFn(), createProjectID, body)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(src)
				return nil
			}
			fmt.Printf("Created source %s (%s/%s)\n", src.ID, src.Provider, src.Repository)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createProjectID, "project", "", "Project ID (required)")
	createCmd.MarkFlagRequired("project")
	createCmd.Flags().StringVar(&createProvider, "provider", "", "Provider: dockerhub, github, ecr, gitlab, pypi, npm (required)")
	createCmd.MarkFlagRequired("provider")
	createCmd.Flags().StringVar(&createRepo, "repository", "", "Repository identifier (required)")
	createCmd.MarkFlagRequired("repository")
	createCmd.Flags().IntVar(&createPollInterval, "poll-interval", 0, "Poll interval in seconds")
	createCmd.Flags().StringVar(&createFilterInclude, "filter-include", "", "Version include regex pattern")
	createCmd.Flags().StringVar(&createFilterExclude, "filter-exclude", "", "Version exclude regex pattern")
	createCmd.Flags().BoolVar(&createExcludePrerelease, "exclude-prereleases", false, "Exclude prerelease versions")

	// --- update ---
	var updateProvider, updateRepo, updateFilterInclude, updateFilterExclude string
	var updatePollInterval int
	var updateEnabled, updateExcludePrerelease string
	updateCmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update a source",
		Args:    cobra.ExactArgs(1),
		Example: "  clog sources update src-123 --repository library/redis",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("provider") {
				fields["provider"] = updateProvider
			}
			if cmd.Flags().Changed("repository") {
				fields["repository"] = updateRepo
			}
			if cmd.Flags().Changed("poll-interval") {
				fields["poll_interval_seconds"] = updatePollInterval
			}
			if cmd.Flags().Changed("enabled") {
				if updateEnabled != "true" && updateEnabled != "false" {
					return fmt.Errorf("--enabled must be 'true' or 'false'")
				}
				fields["enabled"] = updateEnabled == "true"
			}
			if cmd.Flags().Changed("filter-include") {
				fields["version_filter_include"] = updateFilterInclude
			}
			if cmd.Flags().Changed("filter-exclude") {
				fields["version_filter_exclude"] = updateFilterExclude
			}
			if cmd.Flags().Changed("exclude-prereleases") {
				if updateExcludePrerelease != "true" && updateExcludePrerelease != "false" {
					return fmt.Errorf("--exclude-prereleases must be 'true' or 'false'")
				}
				fields["exclude_prereleases"] = updateExcludePrerelease == "true"
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update")
			}
			src, err := UpdateSource(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(src)
				return nil
			}
			fmt.Printf("Updated source %s\n", src.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateProvider, "provider", "", "Provider")
	updateCmd.Flags().StringVar(&updateRepo, "repository", "", "Repository")
	updateCmd.Flags().IntVar(&updatePollInterval, "poll-interval", 0, "Poll interval in seconds")
	updateCmd.Flags().StringVar(&updateEnabled, "enabled", "", "Enable/disable (true/false)")
	updateCmd.Flags().StringVar(&updateFilterInclude, "filter-include", "", "Version include regex")
	updateCmd.Flags().StringVar(&updateFilterExclude, "filter-exclude", "", "Version exclude regex")
	updateCmd.Flags().StringVar(&updateExcludePrerelease, "exclude-prereleases", "", "Exclude prereleases (true/false)")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Remove a source",
		Args:    cobra.ExactArgs(1),
		Example: "  clog sources delete src-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteSource(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted source", args[0])
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}
