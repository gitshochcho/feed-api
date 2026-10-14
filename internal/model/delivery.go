package model

import "time"

// Delivery maps the existing "deliveries" table (owned by another app, never migrated here).
// (asn, store_id) is UNIQUE; store_id is the INTERNAL stores.id.
// Status: planned, preparing, ready, processing, finished. "from" is a reserved word (GORM quotes it).
type Delivery struct {
	ID      uint      `gorm:"primaryKey" json:"id"`
	StoreID uint      `gorm:"column:store_id" json:"store_id"`
	Asn     int64     `gorm:"column:asn" json:"asn"`
	Eta     time.Time `gorm:"column:eta" json:"eta"`
	From    string    `gorm:"column:from" json:"from"`
	Status  string    `gorm:"column:status" json:"status"`
}

func (Delivery) TableName() string { return "deliveries" }
