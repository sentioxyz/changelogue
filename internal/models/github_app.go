package models

import (
	"encoding/json"
	"time"
)

type GitHubAppStatus struct {
	Configured    bool                    `json:"configured"`
	AppID         string                  `json:"app_id,omitempty"`
	InstallURL    string                  `json:"install_url,omitempty"`
	Installations []GitHubAppInstallation `json:"installations"`
}

type GitHubAppInstallation struct {
	ID                  string          `json:"id"`
	InstallationID      int64           `json:"installation_id"`
	AccountLogin        string          `json:"account_login"`
	AccountType         string          `json:"account_type"`
	RepositorySelection string          `json:"repository_selection"`
	Permissions         json.RawMessage `json:"permissions,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type GitHubAppRepository struct {
	InstallationID int64     `json:"installation_id"`
	FullName       string    `json:"full_name"`
	Private        bool      `json:"private"`
	HTMLURL        string    `json:"html_url,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}
