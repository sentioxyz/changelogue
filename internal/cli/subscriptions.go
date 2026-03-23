package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// ListSubscriptions fetches a paginated list of subscriptions.
func ListSubscriptions(c *Client, page, perPage int) ([]models.Subscription, Meta, error) {
	path := fmt.Sprintf("/api/v1/subscriptions?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// GetSubscription fetches a single subscription by ID.
func GetSubscription(c *Client, id string) (*models.Subscription, error) {
	resp, err := c.Get("/api/v1/subscriptions/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateSubscription creates a new subscription.
func CreateSubscription(c *Client, body map[string]any) (*models.Subscription, error) {
	resp, err := c.Post("/api/v1/subscriptions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateSubscription updates an existing subscription with the given fields.
func UpdateSubscription(c *Client, id string, fields map[string]any) (*models.Subscription, error) {
	resp, err := c.Put("/api/v1/subscriptions/"+url.PathEscape(id), fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteSubscription deletes a subscription by ID.
func DeleteSubscription(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/subscriptions/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// BatchCreateSubscriptions creates multiple subscriptions in a single request.
func BatchCreateSubscriptions(c *Client, body map[string]any) ([]models.Subscription, error) {
	resp, err := c.Post("/api/v1/subscriptions/batch", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[[]models.Subscription]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// BatchDeleteSubscriptions deletes multiple subscriptions by ID.
func BatchDeleteSubscriptions(c *Client, ids []string) error {
	body := map[string]any{"ids": ids}
	resp, err := c.DeleteWithBody("/api/v1/subscriptions/batch", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// --- Cobra commands ---

// NewSubscriptionsCmd returns the "subscriptions" command group.
// clientFn is called at execution time to build the client from resolved flags.
func NewSubscriptionsCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Manage subscriptions",
		Long: `Create, list, update, and delete notification subscriptions.

Types:
  source_release    — raw release notifications from a specific source
  semantic_release  — AI-analyzed release notifications for a project

Examples:
  clog subscriptions list
  clog subscriptions create --channel <id> --type source_release --source <id>
  clog subscriptions create --channel <id> --type semantic_release --project <id>`,
	}

	var page, perPage int

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all subscriptions",
		Example: "  clog subscriptions list",
		RunE: func(cmd *cobra.Command, args []string) error {
			subs, meta, err := ListSubscriptions(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": subs, "meta": meta})
				return nil
			}
			rows := make([][]string, len(subs))
			for i, s := range subs {
				target := ""
				if s.SourceID != nil {
					target = "source:" + *s.SourceID
				} else if s.ProjectID != nil {
					target = "project:" + *s.ProjectID
				}
				rows[i] = []string{s.ID, s.ChannelID, s.Type, target, s.VersionFilter}
			}
			RenderTable([]string{"ID", "CHANNEL", "TYPE", "TARGET", "VERSION FILTER"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get subscription details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sub, err := GetSubscription(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(sub)
				return nil
			}
			target := ""
			if sub.SourceID != nil {
				target = "source:" + *sub.SourceID
			} else if sub.ProjectID != nil {
				target = "project:" + *sub.ProjectID
			}
			rows := [][]string{{sub.ID, sub.ChannelID, sub.Type, target, sub.VersionFilter}}
			RenderTable([]string{"ID", "CHANNEL", "TYPE", "TARGET", "VERSION FILTER"}, rows)
			return nil
		},
	}

	// --- create ---
	var createChannelID, createType, createSourceID, createProjectID, createVersionFilter string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a subscription",
		Example: `  clog subscriptions create --channel ch1 --type source_release --source src1
  clog subscriptions create --channel ch1 --type semantic_release --project p1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"channel_id": createChannelID,
				"type":       createType,
			}
			if createSourceID != "" {
				body["source_id"] = createSourceID
			}
			if createProjectID != "" {
				body["project_id"] = createProjectID
			}
			if createVersionFilter != "" {
				body["version_filter"] = createVersionFilter
			}
			sub, err := CreateSubscription(clientFn(), body)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(sub)
				return nil
			}
			fmt.Printf("Created subscription %s\n", sub.ID)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createChannelID, "channel", "", "Channel ID (required)")
	createCmd.MarkFlagRequired("channel")
	createCmd.Flags().StringVar(&createType, "type", "", "Type: source_release or semantic_release (required)")
	createCmd.MarkFlagRequired("type")
	createCmd.Flags().StringVar(&createSourceID, "source", "", "Source ID (for source_release type)")
	createCmd.Flags().StringVar(&createProjectID, "project", "", "Project ID (for semantic_release type)")
	createCmd.Flags().StringVar(&createVersionFilter, "version-filter", "", "Version regex filter")

	// --- update ---
	var updateVersionFilter, updateChannelID string
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("channel") {
				fields["channel_id"] = updateChannelID
			}
			if cmd.Flags().Changed("version-filter") {
				fields["version_filter"] = updateVersionFilter
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update — use --channel or --version-filter")
			}
			sub, err := UpdateSubscription(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(sub)
				return nil
			}
			fmt.Printf("Updated subscription %s\n", sub.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateChannelID, "channel", "", "New channel ID")
	updateCmd.Flags().StringVar(&updateVersionFilter, "version-filter", "", "New version filter")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteSubscription(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted subscription", args[0])
			return nil
		},
	}

	// --- batch-create ---
	var batchChannelID, batchType, batchVersionFilter string
	var batchProjectIDs, batchSourceIDs string
	batchCreateCmd := &cobra.Command{
		Use:     "batch-create",
		Short:   "Batch create subscriptions",
		Example: "  clog subscriptions batch-create --channel ch1 --type semantic_release --project-ids p1,p2,p3",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"channel_id": batchChannelID,
				"type":       batchType,
			}
			if batchProjectIDs != "" {
				body["project_ids"] = strings.Split(batchProjectIDs, ",")
			}
			if batchSourceIDs != "" {
				body["source_ids"] = strings.Split(batchSourceIDs, ",")
			}
			if batchVersionFilter != "" {
				body["version_filter"] = batchVersionFilter
			}
			subs, err := BatchCreateSubscriptions(clientFn(), body)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(subs)
				return nil
			}
			fmt.Printf("Created %d subscriptions\n", len(subs))
			return nil
		},
	}
	batchCreateCmd.Flags().StringVar(&batchChannelID, "channel", "", "Channel ID (required)")
	batchCreateCmd.MarkFlagRequired("channel")
	batchCreateCmd.Flags().StringVar(&batchType, "type", "", "Type: source_release or semantic_release (required)")
	batchCreateCmd.MarkFlagRequired("type")
	batchCreateCmd.Flags().StringVar(&batchProjectIDs, "project-ids", "", "Comma-separated project IDs")
	batchCreateCmd.Flags().StringVar(&batchSourceIDs, "source-ids", "", "Comma-separated source IDs")
	batchCreateCmd.Flags().StringVar(&batchVersionFilter, "version-filter", "", "Version regex filter")

	// --- batch-delete ---
	var batchDeleteIDs string
	batchDeleteCmd := &cobra.Command{
		Use:     "batch-delete",
		Short:   "Batch delete subscriptions",
		Example: "  clog subscriptions batch-delete --ids sub1,sub2,sub3",
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := strings.Split(batchDeleteIDs, ",")
			if err := BatchDeleteSubscriptions(clientFn(), ids); err != nil {
				return err
			}
			fmt.Printf("Deleted %d subscriptions\n", len(ids))
			return nil
		},
	}
	batchDeleteCmd.Flags().StringVar(&batchDeleteIDs, "ids", "", "Comma-separated subscription IDs (required)")
	batchDeleteCmd.MarkFlagRequired("ids")

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd, batchCreateCmd, batchDeleteCmd)
	return cmd
}
