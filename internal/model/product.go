package model

import "time"

// Product maps the existing "products" table (owned by another app, never migrated here).
// vendor_id is deliberately not mapped: the product feed never touches it.
// retail_price, buy_qty, unit_landed_cost, qty_per_parent and promo_price are "real" in the DB.
type Product struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Sku            string    `gorm:"column:sku" json:"sku"`
	CategoryID     uint      `gorm:"column:category_id" json:"category_id"`
	Name           string    `gorm:"column:name" json:"name"`
	Brand          string    `gorm:"column:brand" json:"brand"`
	BrandID        *uint     `gorm:"column:brand_id" json:"brand_id"`
	Status         string    `gorm:"column:status" json:"status"`
	VendorKey      string    `gorm:"column:vendor_key" json:"vendor_key"`
	UomBuy         string    `gorm:"column:uom_buy" json:"uom_buy"`
	BuyQty         float64   `gorm:"column:buy_qty" json:"buy_qty"`
	UomSell        string    `gorm:"column:uom_sell" json:"uom_sell"`
	InnerPack      int       `gorm:"column:inner_pack" json:"inner_pack"`
	MasterPack     int       `gorm:"column:master_pack" json:"master_pack"`
	Image          string    `gorm:"column:image" json:"image"`
	PrimaryUpc     string    `gorm:"column:primary_upc" json:"primary_upc"`
	RetailPrice    float64   `gorm:"column:retail_price" json:"retail_price"`
	UnitLandedCost float64   `gorm:"column:unit_landed_cost" json:"unit_landed_cost"`
	SkuType        string    `gorm:"column:sku_type" json:"sku_type"`
	ParentSku      string    `gorm:"column:parent_sku" json:"parent_sku"`
	QtyPerParent   float64   `gorm:"column:qty_per_parent" json:"qty_per_parent"`
	ReferenceSku   string    `gorm:"column:reference_sku" json:"reference_sku"`
	ShelfDays      int       `gorm:"column:shelf_days" json:"shelf_days"`
	StockTotal     int       `gorm:"column:stock_total" json:"stock_total"`
	StockFs        string    `gorm:"column:stock_fs" json:"stock_fs"`
	StockBs        string    `gorm:"column:stock_bs" json:"stock_bs"`
	Min            int       `gorm:"column:min" json:"min"`
	Max            int       `gorm:"column:max" json:"max"`
	PromoPrice     float64   `gorm:"column:promo_price" json:"promo_price"`
	IsVmi          bool      `gorm:"column:is_vmi" json:"is_vmi"`
	IsFifo         bool      `gorm:"column:is_fifo" json:"is_fifo"`
	IsDeal         bool      `gorm:"column:is_deal" json:"is_deal"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Product) TableName() string { return "products" }
