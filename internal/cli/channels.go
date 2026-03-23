package cli

import (
	"encoding/json"
	"fmt"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// ListChannels fetches a paginated list of notification channels.
func ListChannels(c *Client, page, perPage int) ([]models.NotificationChannel, Meta, error) {
	path := fmt.Sprintf("/api/v1/channels?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// GetChannel fetches a single notification channel by ID.
func GetChannel(c *Client, id string) (*models.NotificationChannel, error) {
	resp, err := c.Get("/api/v1/channels/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateChannel creates a new notification channel.
func CreateChannel(c *Client, name, chType string, config json.RawMessage) (*models.NotificationChannel, error) {
	body := map[string]any{"name": name, "type": chType, "config": config}
	resp, err := c.Post("/api/v1/channels", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateChannel updates an existing notification channel with the given fields.
func UpdateChannel(c *Client, id string, fields map[string]any) (*models.NotificationChannel, error) {
	resp, err := c.Put("/api/v1/channels/"+id, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.NotificationChannel]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteChannel deletes a notification channel by ID.
func DeleteChannel(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/channels/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// TestChannel sends a test notification to a channel.
func TestChannel(c *Client, id string) error {
	resp, err := c.Post("/api/v1/channels/"+id+"/test", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// --- Cobra commands ---

// NewChannelsCmd returns the "channels" command group.
// clientFn is called at execution time to build the client from resolved flags.
func NewChannelsCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage notification channels",
		Long:  "Create, list, update, delete, and test notification channels.\n\nTypes: slack, discord, email, webhook\n\nExamples:\n  clog channels list\n  clog channels create --name my-slack --type slack --config '{\"webhook_url\":\"https://...\"}'\n  clog channels test <id>",
	}

	var page, perPage int

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all notification channels",
		Example: "  clog channels list",
		RunE: func(cmd *cobra.Command, args []string) error {
			channels, meta, err := ListChannels(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": channels, "meta": meta})
				return nil
			}
			rows := make([][]string, len(channels))
			for i, ch := range channels {
				rows[i] = []string{ch.ID, ch.Name, ch.Type, FormatTime(ch.CreatedAt.Format("2006-01-02T15:04:05"))}
			}
			RenderTable([]string{"ID", "NAME", "TYPE", "CREATED"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- get ---
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get channel details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ch, err := GetChannel(clientFn(), args[0])
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(ch)
				return nil
			}
			rows := [][]string{{ch.ID, ch.Name, ch.Type, string(ch.Config)}}
			RenderTable([]string{"ID", "NAME", "TYPE", "CONFIG"}, rows)
			return nil
		},
	}

	// --- create ---
	var createName, createType, createConfig string
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a notification channel",
		Example: "  clog channels create --name my-slack --type slack --config '{\"webhook_url\":\"https://hooks.slack.com/...\"}'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !json.Valid([]byte(createConfig)) {
				return fmt.Errorf("--config must be valid JSON")
			}
			ch, err := CreateChannel(clientFn(), createName, createType, json.RawMessage(createConfig))
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(ch)
				return nil
			}
			fmt.Printf("Created channel %s (%s, type=%s)\n", ch.Name, ch.ID, ch.Type)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Channel name (required)")
	createCmd.MarkFlagRequired("name")
	createCmd.Flags().StringVar(&createType, "type", "", "Channel type: slack, discord, email, webhook (required)")
	createCmd.MarkFlagRequired("type")
	createCmd.Flags().StringVar(&createConfig, "config", "", "Channel config as JSON (required)")
	createCmd.MarkFlagRequired("config")

	// --- update ---
	var updateName, updateType, updateConfig string
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := make(map[string]any)
			if cmd.Flags().Changed("name") {
				fields["name"] = updateName
			}
			if cmd.Flags().Changed("type") {
				fields["type"] = updateType
			}
			if cmd.Flags().Changed("config") {
				if !json.Valid([]byte(updateConfig)) {
					return fmt.Errorf("--config must be valid JSON")
				}
				fields["config"] = json.RawMessage(updateConfig)
			}
			if len(fields) == 0 {
				return fmt.Errorf("no fields to update — use --name, --type, or --config")
			}
			ch, err := UpdateChannel(clientFn(), args[0], fields)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(ch)
				return nil
			}
			fmt.Printf("Updated channel %s\n", ch.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "Channel name")
	updateCmd.Flags().StringVar(&updateType, "type", "", "Channel type")
	updateCmd.Flags().StringVar(&updateConfig, "config", "", "Channel config as JSON")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteChannel(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted channel", args[0])
			return nil
		},
	}

	// --- test ---
	testCmd := &cobra.Command{
		Use:   "test <id>",
		Short: "Send a test notification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := TestChannel(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Test notification sent successfully")
			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd, testCmd)
	return cmd
}
