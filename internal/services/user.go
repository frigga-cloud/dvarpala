// Package services provides user service functionality
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// Errors returned by UserService. Callers should compare with errors.Is
// rather than matching on message text.
var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidEmail = errors.New("invalid email address")
	ErrUserInactive = errors.New("user is not active")
)

// UserService handles user records and their group membership.
type UserService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewUserService creates a user service backed by the given database.
func NewUserService(db *gorm.DB, audit *AuditService) *UserService {
	return &UserService{db: db, audit: audit}
}

// CreateUserRequest is the input to CreateUser.
type CreateUserRequest struct {
	Email      string `json:"email" binding:"required,email"`
	FullName   string `json:"full_name"`
	Department string `json:"department"`
}

// CreateUser adds a user with status "active".
//
// Email is normalised to lower case and must be unique. Note that soft-deleted
// users still occupy their email address, because the unique index covers all
// rows regardless of deleted_at.
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*models.User, error) {
	email := normaliseEmail(req.Email)
	if !looksLikeEmail(email) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidEmail, req.Email)
	}

	var existing models.User
	err := s.db.WithContext(ctx).Unscoped().Where("email = ?", email).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: %s", ErrUserExists, email)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking for existing user: %w", err)
	}

	user := models.User{
		Email:      email,
		FullName:   req.FullName,
		Department: req.Department,
		Status:     models.UserStatusActive,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	s.audit.Log(ctx, Entry{
		UserID:       &user.ID,
		Action:       "user_created",
		ResourceType: "user",
		ResourceID:   &user.ID,
		Details:      map[string]interface{}{"email": user.Email, "department": user.Department},
	})

	return &user, nil
}

// ListUsers returns all users, oldest first, with their groups preloaded.
func (s *UserService) ListUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Preload("Groups").Order("id").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	return users, nil
}

// GetUserByEmail looks up a single user and preloads their groups.
//
// This is the database half of the two-step authentication check: the OAuth
// callback confirms identity, then this confirms authorisation. Returns
// ErrUserNotFound if no such user exists.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Preload("Groups").Where("email = ?", normaliseEmail(email)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrUserNotFound, email)
	}
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}
	return &user, nil
}

// IsAuthorised reports whether a user may be granted VPN access.
//
// A user must exist and have status "active". Being absent from the database
// is a failure even if the identity provider vouched for them - that is the
// point of the second step.
func (s *UserService) IsAuthorised(ctx context.Context, email string) (*models.User, error) {
	user, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user.Status != models.UserStatusActive {
		return nil, fmt.Errorf("%w: %s is %s", ErrUserInactive, email, user.Status)
	}
	return user, nil
}

// DeactivateUser sets a user's status to "inactive", which prevents future
// authentication. It does not terminate sessions that are already open.
func (s *UserService) DeactivateUser(ctx context.Context, email string) error {
	user, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Model(user).Update("status", models.UserStatusInactive).Error; err != nil {
		return fmt.Errorf("deactivating user: %w", err)
	}

	s.audit.Log(ctx, Entry{
		UserID:       &user.ID,
		Action:       "user_deactivated",
		ResourceType: "user",
		ResourceID:   &user.ID,
		Details:      map[string]interface{}{"email": user.Email},
	})

	return nil
}

// RecordLogin stamps the user's last_login time.
func (s *UserService) RecordLogin(ctx context.Context, userID uint) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Update("last_login", &now).Error
}

func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// looksLikeEmail is a deliberately minimal check. Real validation happens at
// the OAuth provider; this only catches obvious mistakes at the CLI.
func looksLikeEmail(email string) bool {
	at := strings.Index(email, "@")
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t")
}
