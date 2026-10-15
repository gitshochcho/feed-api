package model

// StoreProduct maps the existing "store_products" table (owned by another app, never migrated here).
// (store_id, product_id) is UNIQUE. Only the columns the feeds write are mapped;
// on insert the others take their DB defaults (fos_inventory 0, replenishment_type 'filltomax', ...).
type StoreProduct struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	StoreID   uint `gorm:"column:store_id" json:"store_id"`
	ProductID uint `gorm:"column:product_id" json:"product_id"`
	TotalQty  int  `gorm:"column:total_qty" json:"total_qty"`
}

func (StoreProduct) TableName() string { return "store_products" }
