package model

import "time"

// ActivityLog maps the existing "activity_logs" table (owned by another app, never migrated here).
// id is GENERATED ALWAYS, so it is never sent on insert. user_id NULL = written by a feed import.
type ActivityLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    *uint     `gorm:"column:user_id" json:"user_id"`
	StoreID   *uint     `gorm:"column:store_id" json:"store_id"`
	Event     string    `gorm:"column:event" json:"event"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	Metadata  string    `gorm:"column:metadata" json:"metadata"` // jsonb, written as JSON text
}

func (ActivityLog) TableName() string { return "activity_logs" }
