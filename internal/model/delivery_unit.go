package model

// DeliveryUnit maps the existing "delivery_units" table (a pallet; owned by another app, never migrated here).
// (delivery_id, delivery_unit_code) is UNIQUE. processing_* and fos_allocation_percentage are not mapped (stay NULL).
type DeliveryUnit struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	DeliveryID       uint   `gorm:"column:delivery_id" json:"delivery_id"`
	DeliveryUnitCode string `gorm:"column:delivery_unit_code" json:"delivery_unit_code"`
}

func (DeliveryUnit) TableName() string { return "delivery_units" }
