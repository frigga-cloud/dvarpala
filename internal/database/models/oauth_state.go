package models

import (
	"time"
	"gorm.io/gorm"
)

// OAuthState stores OAuth state tokens for CSRF protection during OAuth flows
type OAuthState struct {
	// Primary identifier for the OAuth state record
	ID uint `gorm:"primaryKey"`
	
	// Unique random state token used in OAuth flow for CSRF protection
	State string `gorm:"uniqueIndex;not null;size:255"`
	
	// OAuth provider name (google, microsoft, github)
	Provider string `gorm:"not null;size:50"`
	
	// IP address of user initiating OAuth flow (for additional security)
	UserIP string `gorm:"size:45"`
	
	// Browser user agent of user initiating OAuth flow
	UserAgent string `gorm:"size:500"`
	
	// Timestamp when this state token expires (short-lived for security)
	ExpiresAt time.Time `gorm:"not null"`
	
	// Flag indicating if this state token has been consumed/used
	Used bool `gorm:"default:false"`
	
	// Timestamp when state token was created
	CreatedAt time.Time
	
	// Timestamp when state token was last updated
	UpdatedAt time.Time
	
	// Soft delete timestamp - when state token was cleaned up
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for OAuthState
func (OAuthState) TableName() string {
	return "oauth_states"
}