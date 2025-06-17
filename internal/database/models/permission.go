package models

import (
	"time"
	"gorm.io/gorm"
)

type PermissionType string

const (
	PermissionRead  PermissionType = "read"
	PermissionWrite PermissionType = "write"
	PermissionAdmin PermissionType = "admin"
	PermissionSSH   PermissionType = "ssh"
	PermissionFull  PermissionType = "full"
)

type Permission struct {
	ID         uint           `gorm:"primaryKey"`
	GroupID    uint           `gorm:"not null;index"`
	ResourceID uint           `gorm:"not null;index"`
	Type       PermissionType `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	
	// Associations
	Group    Group    `gorm:"foreignKey:GroupID"`
	Resource Resource `gorm:"foreignKey:ResourceID"`
}