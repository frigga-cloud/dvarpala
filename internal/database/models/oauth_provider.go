// Package models provides OAuth provider configuration model definitions
package models

import (
	"time"
)

// OAuthProvider represents an OAuth provider configuration in the system
type OAuthProvider struct {
	BaseModel

	// OAuth provider name (google, microsoft, github, gitlab)
	Name string `gorm:"uniqueIndex;not null;size:50" json:"name"`
	
	// Display name for the provider in UI
	DisplayName string `gorm:"size:100" json:"display_name"`
	
	// Client ID for OAuth application
	ClientID string `gorm:"not null;size:255" json:"client_id"`
	
	// Client secret for OAuth application (encrypted)
	ClientSecret string `gorm:"not null;size:500" json:"-"`
	
	// OAuth authorization URL
	AuthURL string `gorm:"size:500" json:"auth_url"`
	
	// OAuth token exchange URL  
	TokenURL string `gorm:"size:500" json:"token_url"`
	
	// User info API endpoint
	UserInfoURL string `gorm:"size:500" json:"user_info_url"`
	
	// Redirect URL for OAuth callback
	RedirectURL string `gorm:"not null;size:500" json:"redirect_url"`
	
	// OAuth scopes required
	Scopes string `gorm:"size:255" json:"scopes"`
	
	// Whether this provider is currently enabled
	IsEnabled bool `gorm:"default:true" json:"is_enabled"`
	
	// Provider-specific configuration (JSON)
	Config string `gorm:"type:text" json:"config,omitempty"`
	
	// Order for displaying providers (lower = higher priority)
	DisplayOrder int `gorm:"default:0" json:"display_order"`
	
	// When this provider was last used for authentication
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// TableName returns the table name for OAuthProvider
func (OAuthProvider) TableName() string {
	return "oauth_providers"
}

// OAuthProviderType represents supported OAuth provider types
type OAuthProviderType string

const (
	OAuthProviderGoogle    OAuthProviderType = "google"
	OAuthProviderMicrosoft OAuthProviderType = "microsoft"
	OAuthProviderGitHub    OAuthProviderType = "github"
	OAuthProviderGitLab    OAuthProviderType = "gitlab"
)

// GetDefaultProviderConfig returns default configuration for OAuth providers
func GetDefaultProviderConfig(providerType OAuthProviderType) map[string]string {
	configs := map[OAuthProviderType]map[string]string{
		OAuthProviderGoogle: {
			"auth_url":      "https://accounts.google.com/o/oauth2/auth",
			"token_url":     "https://oauth2.googleapis.com/token",
			"user_info_url": "https://www.googleapis.com/oauth2/v2/userinfo",
			"scopes":        "openid email profile",
			"display_name":  "Google",
		},
		OAuthProviderMicrosoft: {
			"auth_url":      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			"token_url":     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			"user_info_url": "https://graph.microsoft.com/v1.0/me",
			"scopes":        "openid email profile",
			"display_name":  "Microsoft",
		},
		OAuthProviderGitHub: {
			"auth_url":      "https://github.com/login/oauth/authorize",
			"token_url":     "https://github.com/login/oauth/access_token",
			"user_info_url": "https://api.github.com/user",
			"scopes":        "user:email",
			"display_name":  "GitHub",
		},
		OAuthProviderGitLab: {
			"auth_url":      "https://gitlab.com/oauth/authorize",
			"token_url":     "https://gitlab.com/oauth/token",
			"user_info_url": "https://gitlab.com/api/v4/user",
			"scopes":        "read_user",
			"display_name":  "GitLab",
		},
	}
	
	return configs[providerType]
}