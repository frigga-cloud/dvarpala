package database

import (
	"github.com/yourcompany/dvarpala/internal/database/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.Resource{},
		&models.Permission{},
		&models.VPNSession{},
		&models.AuditLog{},
	)
}