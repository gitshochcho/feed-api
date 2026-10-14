package model

import "time"

// ProductVariant maps the existing "product_variant" table (owned by another app, never migrated here).
// (product_id, variant) is UNIQUE: the same variant code can belong to several products.
// status defaults to 'related' in the DB; 'preferred' is set by the app and never by a feed.
type ProductVariant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"column:product_id" json:"product_id"`
	Variant   string    `gorm:"column:variant" json:"variant"`
	Status    string    `gorm:"column:status" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ProductVariant) TableName() string { return "product_variant" }
