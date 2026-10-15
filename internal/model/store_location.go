package model

import "time"

// StoreLocation maps the existing "store_location" table (owned by another app, never migrated here).
// id is GENERATED ALWAYS, so it is never sent on insert. is_back = true is back of store (BOS).
type StoreLocation struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StoreID      uint      `gorm:"column:store_id" json:"store_id"`
	IsBack       bool      `gorm:"column:is_back" json:"is_back"`
	FixtureType  *string   `gorm:"column:fixture_type" json:"fixture_type"`
	Name         *string   `gorm:"column:name" json:"name"`
	DepartmentID *uint     `gorm:"column:department_id" json:"department_id"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (StoreLocation) TableName() string { return "store_location" }
