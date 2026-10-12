package model

import "time"

// Category maps the existing "categories" table (owned by another app, never migrated here).
// The 5 keys together are UNIQUE and NOT NULL ("" means "no key").
// Names and department_id are nullable in the DB, so they are pointers.
type Category struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Level1Key    string    `gorm:"column:level_1_key" json:"level_1_key"`
	Level1Name   *string   `gorm:"column:level_1_name" json:"level_1_name"`
	Level2Key    string    `gorm:"column:level_2_key" json:"level_2_key"`
	Level2Name   *string   `gorm:"column:level_2_name" json:"level_2_name"`
	Level3Key    string    `gorm:"column:level_3_key" json:"level_3_key"`
	Level3Name   *string   `gorm:"column:level_3_name" json:"level_3_name"`
	Level4Key    string    `gorm:"column:level_4_key" json:"level_4_key"`
	Level4Name   *string   `gorm:"column:level_4_name" json:"level_4_name"`
	Level5Key    string    `gorm:"column:level_5_key" json:"level_5_key"`
	Level5Name   *string   `gorm:"column:level_5_name" json:"level_5_name"`
	DepartmentID *uint     `gorm:"column:department_id" json:"department_id"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Category) TableName() string { return "categories" }
