package repository

import (
	"context"

	"gorm.io/gorm"

	"feed-api/internal/model"
)

type VendorRepository struct {
	db *gorm.DB
}

func NewVendorRepository(db *gorm.DB) *VendorRepository {
	return &VendorRepository{db: db}
}

// Create inserts a vendor; GORM fills in ID and CreatedAt.
func (r *VendorRepository) Create(ctx context.Context, vendor *model.Vendor) error {
	return r.db.WithContext(ctx).Create(vendor).Error
}
