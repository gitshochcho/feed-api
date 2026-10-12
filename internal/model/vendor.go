package model

import "time"

type Vendor struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:255;not null;uniqueIndex" json:"name"`
	ExternalID string    `gorm:"size:255;not null;uniqueIndex" json:"external_id"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
}
