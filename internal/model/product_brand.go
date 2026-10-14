package model

import "time"

// ProductBrand maps the existing "product_brands" table (owned by another app, never migrated here).
// Both name and external_id are UNIQUE.
type ProductBrand struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"column:name" json:"name"`
	ExternalID string    `gorm:"column:external_id" json:"external_id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ProductBrand) TableName() string { return "product_brands" }
