package model

import "time"

// StockTake maps the existing "stock_takes" table (owned by another app, never migrated here).
// (store_id, product_id) is UNIQUE: one row per product per store, holding the last count.
type StockTake struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	StoreID   uint      `gorm:"column:store_id" json:"store_id"`
	ProductID uint      `gorm:"column:product_id" json:"product_id"`
	BosQty    int       `gorm:"column:bos_qty" json:"bos_qty"`
	FosQty    int       `gorm:"column:fos_qty" json:"fos_qty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (StockTake) TableName() string { return "stock_takes" }
