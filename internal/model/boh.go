package model

import "time"

// Boh maps the existing "boh" table (back-of-house stock counts; owned by another app, never migrated here).
// (store_id, product_id, date) is UNIQUE. Date is kept as "YYYY-MM-DD" text, so time zones can't shift it.
type Boh struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	StoreID   uint      `gorm:"column:store_id" json:"store_id"`
	ProductID uint      `gorm:"column:product_id" json:"product_id"`
	Qty       int       `gorm:"column:qty" json:"qty"`
	StoreQty  int       `gorm:"column:store_qty" json:"store_qty"`
	Date      string    `gorm:"column:date" json:"date"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Boh) TableName() string { return "boh" }
