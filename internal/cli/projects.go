package cli

import (
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// ListProjects fetches a paginated list of projects.
func ListProjects(c *Client, page, perPage int) ([]models.Project, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// GetProject fetches a single project by ID.
func GetProject(c *Client, id string) (*models.Project, error) {
	resp, err := c.Get("/api/v1/projects/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateProject creates a new project with the given name, description, and agent prompt.
func CreateProject(c *Client, name, description, agentPrompt string) (*models.Project, error) {
	body := map[string]string{"name": name}
	if description != "" {
		body["description"] = description
	}
	if agentPrompt != "" {
		body["agent_prompt"] = agentPrompt
	}
	resp, err := c.Post("/api/v1/projects", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateProject updates an existing project with the given fields.
func UpdateProject(c *Client, id string, fields map[string]any) (*models.Project, error) {
	resp, err := c.Put("/api/v1/projects/"+id, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Project]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteProject deletes a project by ID.
func DeleteProject(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/projects/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// --- Cobra commands ---

// NewProjectsCmd returns the "projects" command group.
// clientFn is called at execution time to build the client from resolved flags.
func NewProjectsCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
		Long:  "Create, list, update, and delete projects.\n\nExamples:\n  clog projects list\n  clog projects get <id>\n  clog projects create --name \"My Project\"",
	}

	// Pagination flags (only apply to list)
	var page, perPage int

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all projects",
		Example: "  clog projects list\n  clog projects list --page 2 --per-page 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, meta, err := ListProjects(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": projects, "meta": meta})
				return nil
			}
			rows := make([][]string, len(projects))
			for i, p := range projects {
				rows[i] = []string{p.ID, p.Name, Truncate(p.Description, 40), FormatTime(p.CreatedAt.Format("2006-01-02T15:04:05"))}
			}
			RenderTable([]string{"ID", "NAME", "DESCRIPTION", "CREATED"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Get project details",
		Args:    cobra.ExactArgs(1),
		Example: "  clog projects get abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := GetProject(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(proj)
				return nil
			}
			rows := [][]string{{proj.ID, proj.Name, Truncate(proj.Description, 40), FormatTime(proj.CreatedAt.Format("2006-01-02T15:04:05"))}}
			RenderTable([]string{"ID", "NAME", "DESCRIPTION", "CREATED"}, rows)
			return nil
		},
	}

	// --- create ---
	var createName, createDesc, createPrompt string
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new project",
		Example: "  clog projects create --name \"My Project\" --description \"Tracks postgres releases\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := CreateProject(clientFn(), createName, createDesc, createPrompt)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(proj)
				return nil
			}
			fmt.Printf("Created project %s (%s)\n", proj.Name, proj.ID)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Project name (required)")
	createCmd.MarkFlagRequired("name")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Project description")
	createCmd.Flags().StringVar(&createPrompt, "agent-prompt", "", "Agent prompt for AI analysis")

	// --- update ---
	var updateName, updateDesc, updatePrompt string
	updateCmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update a project",
		Args:    cobra.ExactArgs(1),
		Example: "  clog projects update abc-123 --name \"New Name\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("name") {
				fields["name"] = updateName
			}
			if cmd.Flags().Changed("description") {
				fields["description"] = updateDesc
			}
			if cmd.Flags().Changed("agent-prompt") {
				fields["agent_prompt"] = updatePrompt
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update — use --name, --description, or --agent-prompt")
			}
			proj, err := UpdateProject(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(proj)
				return nil
			}
			fmt.Printf("Updated project %s (%s)\n", proj.Name, proj.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "New project name")
	updateCmd.Flags().StringVar(&updateDesc, "description", "", "New description")
	updateCmd.Flags().StringVar(&updatePrompt, "agent-prompt", "", "New agent prompt")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a project",
		Args:    cobra.ExactArgs(1),
		Example: "  clog projects delete abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteProject(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted project", args[0])
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}
