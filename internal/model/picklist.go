package model

import "time"

// Picklist maps the existing "picklists" table (owned by another app, never migrated here).
// Status: N new, C completed, others = ongoing. Type "advanced" is what the feeds create.
type Picklist struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Name              string    `gorm:"column:name" json:"name"`
	StoreID           *uint     `gorm:"column:store_id" json:"store_id"`
	Sequence          int       `gorm:"column:sequence" json:"sequence"`
	Status            string    `gorm:"column:status" json:"status"`
	Type              string    `gorm:"column:type" json:"type"`
	IsUrgent          bool      `gorm:"column:is_urgent" json:"is_urgent"`
	IsLocked          bool      `gorm:"column:is_locked" json:"is_locked"`
	ReplenishmentType string    `gorm:"column:replenishment_type" json:"replenishment_type"`
	DeliveryID        *uint     `gorm:"column:delivery_id" json:"delivery_id"`
	DepartmentID      *uint     `gorm:"column:department_id" json:"department_id"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Picklist) TableName() string { return "picklists" }

// PicklistProduct maps the existing "picklist_products" table (one item of a picklist).
// Status: O open, F finished. Only the columns the POS feed writes are mapped.
type PicklistProduct struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	PicklistID       uint   `gorm:"column:picklist_id" json:"picklist_id"`
	ProductID        uint   `gorm:"column:product_id" json:"product_id"`
	Status           string `gorm:"column:status" json:"status"`
	DesiredQuantity  int    `gorm:"column:desired_quantity" json:"desired_quantity"`
	PickedupQuantity int    `gorm:"column:pickedup_quantity" json:"pickedup_quantity"`
	PickedupCases    int    `gorm:"column:pickedup_cases" json:"pickedup_cases"`
}

func (PicklistProduct) TableName() string { return "picklist_products" }
