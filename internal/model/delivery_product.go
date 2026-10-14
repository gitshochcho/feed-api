package model

// DeliveryProduct maps the existing "delivery_products" table (owned by another app, never migrated here).
// (delivery_unit_id, product_id) is UNIQUE. damaged_cases and description are nullable, so they are pointers.
// delivered_cases, delivered_units and verified_by are not mapped (stay NULL).
type DeliveryProduct struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	DeliveryUnitID uint    `gorm:"column:delivery_unit_id" json:"delivery_unit_id"`
	ProductID      uint    `gorm:"column:product_id" json:"product_id"`
	ExpectedCases  int     `gorm:"column:expected_cases" json:"expected_cases"`
	ExpectedUnits  int     `gorm:"column:expected_units" json:"expected_units"`
	DamagedCases   *int    `gorm:"column:damaged_cases" json:"damaged_cases"`
	Description    *string `gorm:"column:description" json:"description"`
}

func (DeliveryProduct) TableName() string { return "delivery_products" }
