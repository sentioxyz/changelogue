package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// GetGate fetches the release gate configuration for a project.
func GetGate(c *Client, projectID string) (*models.ReleaseGate, error) {
	resp, err := c.Get(fmt.Sprintf("/api/v1/projects/%s/release-gate", url.PathEscape(projectID)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.ReleaseGate]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpsertGate creates or updates the release gate for a project.
func UpsertGate(c *Client, projectID string, body map[string]any) (*models.ReleaseGate, error) {
	resp, err := c.Put(fmt.Sprintf("/api/v1/projects/%s/release-gate", url.PathEscape(projectID)), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.ReleaseGate]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteGate removes the release gate configuration for a project.
func DeleteGate(c *Client, projectID string) error {
	resp, err := c.Delete(fmt.Sprintf("/api/v1/projects/%s/release-gate", url.PathEscape(projectID)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// ListReadiness fetches paginated version readiness entries for a project.
func ListReadiness(c *Client, projectID string, page, perPage int) ([]models.VersionReadiness, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/version-readiness?page=%d&per_page=%d",
		url.PathEscape(projectID), page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.VersionReadiness]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// ListGateEvents fetches paginated gate events for a project.
func ListGateEvents(c *Client, projectID string, page, perPage int) ([]models.GateEvent, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/gate-events?page=%d&per_page=%d",
		url.PathEscape(projectID), page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.GateEvent]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// ListGateEventsByVersion fetches paginated gate events for a specific version.
func ListGateEventsByVersion(c *Client, projectID, version string, page, perPage int) ([]models.GateEvent, Meta, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/version-readiness/%s/events?page=%d&per_page=%d",
		url.PathEscape(projectID), url.PathEscape(version), page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.GateEvent]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// --- Cobra commands ---

// NewGatesCmd returns the "gates" command group.
func NewGatesCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gates",
		Short: "Manage release gates",
		Long: `Configure release gates, view version readiness, and inspect gate events.

Release gates delay agent analysis until all required sources release the same version.

Examples:
  clog gates get <project-id>
  clog gates set <project-id> --enabled --timeout 168
  clog gates delete <project-id>
  clog gates enable <project-id>
  clog gates disable <project-id>
  clog gates readiness <project-id>
  clog gates events <project-id>`,
	}

	// --- get ---
	getCmd := &cobra.Command{
		Use:     "get <project-id>",
		Short:   "Get release gate configuration",
		Args:    cobra.ExactArgs(1),
		Example: "  clog gates get abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			gate, err := GetGate(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(gate)
				return nil
			}
			sources := "(all)"
			if len(gate.RequiredSources) > 0 {
				sources = strings.Join(gate.RequiredSources, ", ")
			}
			enabledStr := "no"
			if gate.Enabled {
				enabledStr = "yes"
			}
			rows := [][]string{{
				gate.ID,
				enabledStr,
				fmt.Sprintf("%d", gate.TimeoutHours),
				sources,
				Truncate(gate.NLRule, 40),
				FormatTime(gate.UpdatedAt.Format("2006-01-02T15:04:05")),
			}}
			RenderTable([]string{"ID", "ENABLED", "TIMEOUT (H)", "REQUIRED SOURCES", "NL RULE", "UPDATED"}, rows)
			return nil
		},
	}

	// --- set ---
	var (
		setEnabled         bool
		setDisabled        bool
		setTimeout         int
		setRequiredSources string
		setNLRule          string
	)
	setCmd := &cobra.Command{
		Use:   "set <project-id>",
		Short: "Create or update release gate configuration",
		Args:  cobra.ExactArgs(1),
		Example: `  clog gates set abc-123 --enabled --timeout 168
  clog gates set abc-123 --required-sources "src-1,src-2" --nl-rule "Only stable tags"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("enabled") {
				fields["enabled"] = true
			}
			if cmd.Flags().Changed("disabled") {
				fields["enabled"] = false
			}
			if cmd.Flags().Changed("timeout") {
				fields["timeout_hours"] = setTimeout
			}
			if cmd.Flags().Changed("required-sources") {
				if setRequiredSources == "" {
					fields["required_sources"] = []string{}
				} else {
					fields["required_sources"] = strings.Split(setRequiredSources, ",")
				}
			}
			if cmd.Flags().Changed("nl-rule") {
				fields["nl_rule"] = setNLRule
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to set — use --enabled, --disabled, --timeout, --required-sources, or --nl-rule")
			}
			gate, err := UpsertGate(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(gate)
				return nil
			}
			fmt.Printf("Release gate updated for project %s\n", args[0])
			return nil
		},
	}
	setCmd.Flags().BoolVar(&setEnabled, "enabled", false, "Enable the gate")
	setCmd.Flags().BoolVar(&setDisabled, "disabled", false, "Disable the gate")
	setCmd.Flags().IntVar(&setTimeout, "timeout", 168, "Timeout in hours")
	setCmd.Flags().StringVar(&setRequiredSources, "required-sources", "", "Comma-separated source IDs (empty = all)")
	setCmd.Flags().StringVar(&setNLRule, "nl-rule", "", "Natural language rule")
	setCmd.MarkFlagsMutuallyExclusive("enabled", "disabled")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:     "delete <project-id>",
		Short:   "Delete release gate configuration",
		Args:    cobra.ExactArgs(1),
		Example: "  clog gates delete abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteGate(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Release gate deleted for project %s\n", args[0])
			return nil
		},
	}

	// --- enable ---
	enableCmd := &cobra.Command{
		Use:     "enable <project-id>",
		Short:   "Enable the release gate",
		Args:    cobra.ExactArgs(1),
		Example: "  clog gates enable abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			gate, err := UpsertGate(clientFn(), args[0], map[string]any{"enabled": true})
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(gate)
				return nil
			}
			fmt.Printf("Release gate enabled for project %s\n", args[0])
			return nil
		},
	}

	// --- disable ---
	disableCmd := &cobra.Command{
		Use:     "disable <project-id>",
		Short:   "Disable the release gate",
		Args:    cobra.ExactArgs(1),
		Example: "  clog gates disable abc-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			gate, err := UpsertGate(clientFn(), args[0], map[string]any{"enabled": false})
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(gate)
				return nil
			}
			fmt.Printf("Release gate disabled for project %s\n", args[0])
			return nil
		},
	}

	// --- readiness ---
	var readPage, readPerPage int
	readinessCmd := &cobra.Command{
		Use:     "readiness <project-id>",
		Short:   "List version readiness status",
		Args:    cobra.ExactArgs(1),
		Example: "  clog gates readiness abc-123\n  clog gates readiness abc-123 --page 2",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, meta, err := ListReadiness(clientFn(), args[0], readPage, readPerPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": entries, "meta": meta})
				return nil
			}
			rows := make([][]string, len(entries))
			for i, vr := range entries {
				met := strings.Join(vr.SourcesMet, ", ")
				if met == "" {
					met = "-"
				}
				missing := strings.Join(vr.SourcesMissing, ", ")
				if missing == "" {
					missing = "-"
				}
				rows[i] = []string{
					vr.Version,
					vr.Status,
					Truncate(met, 30),
					Truncate(missing, 30),
					FormatTime(vr.TimeoutAt.Format("2006-01-02T15:04:05")),
				}
			}
			RenderTable([]string{"VERSION", "STATUS", "SOURCES MET", "SOURCES MISSING", "TIMEOUT AT"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	readinessCmd.Flags().IntVar(&readPage, "page", 1, "Page number")
	readinessCmd.Flags().IntVar(&readPerPage, "per-page", 25, "Items per page")

	// --- events ---
	var evtPage, evtPerPage int
	var evtVersion string
	eventsCmd := &cobra.Command{
		Use:     "events <project-id>",
		Short:   "List gate events",
		Args:    cobra.ExactArgs(1),
		Example: "  clog gates events abc-123\n  clog gates events abc-123 --version v1.2.0",
		RunE: func(cmd *cobra.Command, args []string) error {
			var events []models.GateEvent
			var meta Meta
			var err error
			if evtVersion != "" {
				events, meta, err = ListGateEventsByVersion(clientFn(), args[0], evtVersion, evtPage, evtPerPage)
			} else {
				events, meta, err = ListGateEvents(clientFn(), args[0], evtPage, evtPerPage)
			}
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": events, "meta": meta})
				return nil
			}
			rows := make([][]string, len(events))
			for i, ev := range events {
				sourceID := "-"
				if ev.SourceID != nil {
					sourceID = *ev.SourceID
				}
				rows[i] = []string{
					ev.Version,
					ev.EventType,
					Truncate(sourceID, 20),
					FormatTime(ev.CreatedAt.Format("2006-01-02T15:04:05")),
				}
			}
			RenderTable([]string{"VERSION", "EVENT TYPE", "SOURCE", "TIMESTAMP"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	eventsCmd.Flags().IntVar(&evtPage, "page", 1, "Page number")
	eventsCmd.Flags().IntVar(&evtPerPage, "per-page", 25, "Items per page")
	eventsCmd.Flags().StringVar(&evtVersion, "version", "", "Filter events by version")

	cmd.AddCommand(getCmd, setCmd, deleteCmd, enableCmd, disableCmd, readinessCmd, eventsCmd)
	return cmd
}
