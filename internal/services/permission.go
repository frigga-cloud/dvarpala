// Package services provides permission service functionality
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// Errors returned by PermissionService.
var (
	ErrInvalidPermission = errors.New("invalid permission type")
)

// PermissionService grants and revokes a group's access to a resource, and
// answers the question the VPN ultimately needs: what may this person reach?
type PermissionService struct {
	db        *gorm.DB
	audit     *AuditService
	users     *UserService
	groups    *GroupService
	resources *ResourceService
}

// NewPermissionService creates a permission service.
func NewPermissionService(db *gorm.DB, audit *AuditService, users *UserService,
	groups *GroupService, resources *ResourceService) *PermissionService {
	return &PermissionService{
		db: db, audit: audit, users: users, groups: groups, resources: resources,
	}
}

// validPermissionTypes mirrors models.PermissionType.
var validPermissionTypes = map[string]models.PermissionType{
	"read":  models.PermissionRead,
	"write": models.PermissionWrite,
	"admin": models.PermissionAdmin,
	"ssh":   models.PermissionSSH,
	"full":  models.PermissionFull,
}

// Grant gives a group a level of access to a resource.
//
// Granting the same permission twice is not an error: the row's primary key is
// (group, resource, type), so re-granting is a no-op.
func (s *PermissionService) Grant(ctx context.Context, groupName, resourceName, permission string) error {
	kind, ok := validPermissionTypes[strings.ToLower(strings.TrimSpace(permission))]
	if !ok {
		return fmt.Errorf("%w: must be one of read, write, admin, ssh, full (got %q)",
			ErrInvalidPermission, permission)
	}

	group, err := s.groups.GetGroupByName(ctx, groupName)
	if err != nil {
		return err
	}
	resource, err := s.resources.GetResourceByName(ctx, resourceName)
	if err != nil {
		return err
	}

	gp := models.GroupPermission{
		GroupID:        group.ID,
		ResourceID:     resource.ID,
		PermissionType: kind,
	}
	// The composite primary key makes a repeated grant harmless.
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND resource_id = ? AND permission_type = ?",
			group.ID, resource.ID, kind).
		FirstOrCreate(&gp).Error; err != nil {
		return fmt.Errorf("granting %s on %s to %s: %w", kind, resourceName, groupName, err)
	}

	s.audit.Log(ctx, Entry{
		Action:       "permission_granted",
		ResourceType: "resource",
		ResourceID:   &resource.ID,
		Details: map[string]interface{}{
			"group": group.Name, "resource": resource.Name, "permission": string(kind),
		},
	})

	return nil
}

// Revoke removes a group's permission on a resource.
func (s *PermissionService) Revoke(ctx context.Context, groupName, resourceName, permission string) error {
	kind, ok := validPermissionTypes[strings.ToLower(strings.TrimSpace(permission))]
	if !ok {
		return fmt.Errorf("%w: %q", ErrInvalidPermission, permission)
	}

	group, err := s.groups.GetGroupByName(ctx, groupName)
	if err != nil {
		return err
	}
	resource, err := s.resources.GetResourceByName(ctx, resourceName)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND resource_id = ? AND permission_type = ?",
			group.ID, resource.ID, kind).
		Delete(&models.GroupPermission{}).Error; err != nil {
		return fmt.Errorf("revoking: %w", err)
	}

	s.audit.Log(ctx, Entry{
		Action:       "permission_revoked",
		ResourceType: "resource",
		ResourceID:   &resource.ID,
		Details: map[string]interface{}{
			"group": group.Name, "resource": resource.Name, "permission": string(kind),
		},
	})

	return nil
}

// Grant describes one resource a subject may reach, and how.
type AccessGrant struct {
	Resource   models.Resource
	Permission models.PermissionType
	ViaGroup   string
}

// ListForGroup returns everything a group may reach.
func (s *PermissionService) ListForGroup(ctx context.Context, groupName string) ([]AccessGrant, error) {
	group, err := s.groups.GetGroupByName(ctx, groupName)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		models.Resource
		PermissionType models.PermissionType
	}
	err = s.db.WithContext(ctx).
		Table("group_permissions gp").
		Select("r.*, gp.permission_type").
		Joins("JOIN resources r ON r.id = gp.resource_id AND r.deleted_at IS NULL").
		Where("gp.group_id = ?", group.ID).
		Order("r.id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing permissions for %s: %w", groupName, err)
	}

	grants := make([]AccessGrant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, AccessGrant{
			Resource: row.Resource, Permission: row.PermissionType, ViaGroup: group.Name,
		})
	}
	return grants, nil
}

// ResourcesForUser returns everything a user may reach, through any group they
// belong to.
//
// This is the question the VPN asks after authentication: it determines which
// routes get pushed to that client (phase 4). A user in no groups reaches
// nothing, which is the intended default.
//
// TODO(phase-2): inherited permissions. Groups have a parent_id, so a child
// group should also receive its ancestors' grants. Currently only direct
// membership is considered.
func (s *PermissionService) ResourcesForUser(ctx context.Context, email string) ([]AccessGrant, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		models.Resource
		PermissionType models.PermissionType
		GroupName      string
	}
	err = s.db.WithContext(ctx).
		Table("user_groups ug").
		Select("r.*, gp.permission_type, g.name AS group_name").
		Joins("JOIN group_permissions gp ON gp.group_id = ug.group_id").
		Joins("JOIN groups g ON g.id = ug.group_id AND g.deleted_at IS NULL").
		Joins("JOIN resources r ON r.id = gp.resource_id AND r.deleted_at IS NULL").
		Where("ug.user_id = ?", user.ID).
		Order("r.id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("resolving access for %s: %w", email, err)
	}

	grants := make([]AccessGrant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, AccessGrant{
			Resource: row.Resource, Permission: row.PermissionType, ViaGroup: row.GroupName,
		})
	}
	return grants, nil
}
