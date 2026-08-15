// Package services provides group service functionality
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// Errors returned by GroupService.
var (
	ErrGroupNotFound = errors.New("group not found")
	ErrGroupExists   = errors.New("group already exists")
	ErrInvalidGroup  = errors.New("invalid group name")
)

// GroupService handles groups and their membership.
type GroupService struct {
	db    *gorm.DB
	audit *AuditService
	users *UserService
}

// NewGroupService creates a group service. It depends on UserService so that
// membership changes resolve users the same way every other caller does.
func NewGroupService(db *gorm.DB, audit *AuditService, users *UserService) *GroupService {
	return &GroupService{db: db, audit: audit, users: users}
}

// CreateGroupRequest is the input to CreateGroup.
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`

	// Optional parent, by name, for hierarchical groups.
	Parent string `json:"parent"`
}

// CreateGroup adds a group, optionally beneath a parent.
func (s *GroupService) CreateGroup(ctx context.Context, req CreateGroupRequest) (*models.Group, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidGroup)
	}

	var existing models.Group
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: %s", ErrGroupExists, name)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking for existing group: %w", err)
	}

	group := models.Group{Name: name, Description: req.Description}

	if req.Parent != "" {
		parent, err := s.GetGroupByName(ctx, req.Parent)
		if err != nil {
			return nil, fmt.Errorf("resolving parent: %w", err)
		}
		group.ParentID = &parent.ID
	}

	if err := s.db.WithContext(ctx).Create(&group).Error; err != nil {
		return nil, fmt.Errorf("creating group: %w", err)
	}

	s.audit.Log(ctx, Entry{
		Action:       "group_created",
		ResourceType: "group",
		ResourceID:   &group.ID,
		Details:      map[string]interface{}{"name": group.Name, "parent": req.Parent},
	})

	return &group, nil
}

// ListGroups returns all groups with their members preloaded.
func (s *GroupService) ListGroups(ctx context.Context) ([]models.Group, error) {
	var groups []models.Group
	if err := s.db.WithContext(ctx).Preload("Users").Order("id").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	return groups, nil
}

// GetGroupByName looks up one group, with members preloaded.
func (s *GroupService) GetGroupByName(ctx context.Context, name string) (*models.Group, error) {
	var group models.Group
	err := s.db.WithContext(ctx).Preload("Users").
		Where("name = ?", strings.TrimSpace(name)).First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrGroupNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("looking up group: %w", err)
	}
	return &group, nil
}

// AddUserToGroup makes a user a member of a group.
//
// Adding an existing member is not an error - the operation is idempotent, so
// scripts and bulk imports can run repeatedly without special handling.
func (s *GroupService) AddUserToGroup(ctx context.Context, email, groupName string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	group, err := s.GetGroupByName(ctx, groupName)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Model(user).
		Association("Groups").Append(group); err != nil {
		return fmt.Errorf("adding %s to %s: %w", email, groupName, err)
	}

	s.audit.Log(ctx, Entry{
		UserID:       &user.ID,
		Action:       "user_added_to_group",
		ResourceType: "group",
		ResourceID:   &group.ID,
		Details:      map[string]interface{}{"email": user.Email, "group": group.Name},
	})

	return nil
}

// RemoveUserFromGroup removes a membership. Removing a non-member is not an error.
func (s *GroupService) RemoveUserFromGroup(ctx context.Context, email, groupName string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	group, err := s.GetGroupByName(ctx, groupName)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Model(user).
		Association("Groups").Delete(group); err != nil {
		return fmt.Errorf("removing %s from %s: %w", email, groupName, err)
	}

	s.audit.Log(ctx, Entry{
		UserID:       &user.ID,
		Action:       "user_removed_from_group",
		ResourceType: "group",
		ResourceID:   &group.ID,
		Details:      map[string]interface{}{"email": user.Email, "group": group.Name},
	})

	return nil
}
