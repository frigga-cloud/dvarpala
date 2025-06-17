// Package config provides OAuth configuration functionality
package config

// GitLabProvider represents GitLab OAuth configuration with custom URL support
type GitLabProvider struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
	URL          string `mapstructure:"url"` // Custom GitLab instance URL
}
