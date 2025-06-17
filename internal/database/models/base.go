// Package models provides base database model definitions
package models

import (
	"time"
	"gorm.io/gorm"
)

// BaseModel contains common columns for all tables with standard timestamps and soft delete
type BaseModel struct {
	// Primary identifier for the record
	ID uint `gorm:"primaryKey"`
	
	// Timestamp when record was created (automatically set by GORM)
	CreatedAt time.Time
	
	// Timestamp when record was last updated (automatically set by GORM)
	UpdatedAt time.Time
	
	// Soft delete timestamp - when record was logically deleted
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableNamer interface for models that want to specify custom table names
type TableNamer interface {
	TableName() string
}
