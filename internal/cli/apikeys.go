package cli

import (
	"fmt"
	"net/url"

	"github.com/sentioxyz/changelogue/internal/models"
	"github.com/spf13/cobra"
)

// --- API functions ---

// ListApiKeys fetches a paginated list of API keys.
func ListApiKeys(c *Client, page, perPage int) ([]models.ApiKey, Meta, error) {
	path := fmt.Sprintf("/api/v1/api-keys?page=%d&per_page=%d", page, perPage)
	resp, err := c.Get(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, Meta{}, err
	}
	var result APIResponse[[]models.ApiKey]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, Meta{}, err
	}
	return result.Data, result.Meta, nil
}

// CreateApiKey creates a new API key with the given name.
func CreateApiKey(c *Client, name string) (*models.ApiKey, error) {
	body := map[string]any{"name": name}
	resp, err := c.Post("/api/v1/api-keys", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	var result APIResponse[models.ApiKey]
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeleteApiKey deletes an API key by ID.
func DeleteApiKey(c *Client, id string) error {
	resp, err := c.Delete("/api/v1/api-keys/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return CheckResponse(resp)
}

// --- Cobra commands ---

// NewApiKeysCmd returns the "api-keys" command group.
func NewApiKeysCmd(clientFn func() *Client, jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-keys",
		Short: "Manage API keys",
		Long:  "Create, list, and delete API keys for programmatic access.\n\nExamples:\n  clog api-keys create --name my-key\n  clog api-keys list\n  clog api-keys delete <id>",
	}

	var page, perPage int

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List all API keys",
		Example: "  clog api-keys list",
		RunE: func(cmd *cobra.Command, args []string) error {
			keys, meta, err := ListApiKeys(clientFn(), page, perPage)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(map[string]any{"data": keys, "meta": meta})
				return nil
			}
			rows := make([][]string, len(keys))
			for i, k := range keys {
				lastUsed := "Never"
				if k.LastUsedAt != nil {
					lastUsed = FormatTime(k.LastUsedAt.Format("2006-01-02T15:04:05"))
				}
				rows[i] = []string{k.ID, k.Name, k.KeyPrefix + "...", lastUsed, FormatTime(k.CreatedAt.Format("2006-01-02T15:04:05"))}
			}
			RenderTable([]string{"ID", "NAME", "PREFIX", "LAST USED", "CREATED"}, rows)
			fmt.Printf("\nShowing page %d (total: %d)\n", meta.Page, meta.Total)
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "Page number")
	listCmd.Flags().IntVar(&perPage, "per-page", 25, "Items per page")

	// --- create ---
	var createName string
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new API key",
		Example: "  clog api-keys create --name my-key",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := CreateApiKey(clientFn(), createName)
			if err != nil {
				return err
			}
			if *jsonFlag {
				RenderJSON(key)
				return nil
			}
			fmt.Printf("Created API key: %s\n", key.Name)
			fmt.Printf("Key: %s\n", key.Key)
			fmt.Println("\nSave this key — it will not be shown again.")
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Key name (required)")
	createCmd.MarkFlagRequired("name")

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteApiKey(clientFn(), args[0]); err != nil {
				return err
			}
			fmt.Println("Deleted API key", args[0])
			return nil
		},
	}

	cmd.AddCommand(listCmd, createCmd, deleteCmd)
	return cmd
}
