package model

// Store maps the existing "stores" table (owned by another app, never migrated here).
// Feeds send external_id; every other table uses stores.id. They are not always equal.
type Store struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ExternalID int64  `gorm:"column:external_id" json:"external_id"`
	Name       string `gorm:"column:name" json:"name"`
}

func (Store) TableName() string { return "stores" }
