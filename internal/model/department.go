package model

import "time"

// Department maps the existing "departments" table (owned by another app, never migrated here).
// level and parent_id are nullable in the DB, so they are pointers.
type Department struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ExternalID string    `gorm:"column:external_id" json:"external_id"`
	Name       string    `gorm:"column:name" json:"name"`
	Level      *int      `gorm:"column:level" json:"level"`
	ParentID   *uint     `gorm:"column:parent_id" json:"parent_id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Department) TableName() string { return "departments" }
