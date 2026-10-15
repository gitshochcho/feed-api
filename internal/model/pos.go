package model

import "time"

// Pos maps the existing "pos" table (one sale line; owned by another app, never migrated here).
// (store_id, register, product_id, transaction) is UNIQUE. qty, prices and discount_code are "real".
// "transaction" is a reserved word in some databases; GORM quotes column names.
type Pos struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	StoreID          uint      `gorm:"column:store_id" json:"store_id"`
	ProductID        uint      `gorm:"column:product_id" json:"product_id"`
	TransactedAt     time.Time `gorm:"column:transacted_at" json:"transacted_at"`
	Register         int       `gorm:"column:register" json:"register"`
	Operator         int       `gorm:"column:operator" json:"operator"`
	TransactionType  string    `gorm:"column:transaction_type" json:"transaction_type"`
	Transaction      int       `gorm:"column:transaction" json:"transaction"`
	SeqNumber        int       `gorm:"column:seq_number" json:"seq_number"`
	Sku              string    `gorm:"column:sku" json:"sku"`
	Qty              float64   `gorm:"column:qty" json:"qty"`
	RegularPriceEach float64   `gorm:"column:regular_price_each" json:"regular_price_each"`
	SoldPriceEach    float64   `gorm:"column:sold_price_each" json:"sold_price_each"`
	IsPriceOverride  bool      `gorm:"column:is_price_override" json:"is_price_override"`
	DiscountCode     float64   `gorm:"column:discount_code" json:"discount_code"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Pos) TableName() string { return "pos" }
