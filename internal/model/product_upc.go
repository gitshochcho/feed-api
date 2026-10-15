package model

// ProductUpc maps the existing "product_upcs" table (owned by another app, never migrated here).
// upc is UNIQUE (the index is named product_upcs_sku_unique): one UPC belongs to one product,
// one product can have many UPCs. The table has no created_at.
type ProductUpc struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ProductID uint   `gorm:"column:product_id" json:"product_id"`
	Upc       string `gorm:"column:upc" json:"upc"`
}

func (ProductUpc) TableName() string { return "product_upcs" }
