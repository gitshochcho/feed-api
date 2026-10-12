package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

type VendorRepository struct {
	db *gorm.DB
}

func NewVendorRepository(db *gorm.DB) *VendorRepository {
	return &VendorRepository{db: db}
}

// Upsert saves one vendor and links it to every store, all in ONE transaction.
// If any step fails, nothing is saved.
func (r *VendorRepository) Upsert(ctx context.Context, name, externalID string) (*dto.VendorUpsertResult, error) {
	result := &dto.VendorUpsertResult{}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// -----------------------------------------------------------------
		// STEP 1 — VENDOR (upsert keyed by external_id, only name is updated)
		//   not found       => INSERT
		//   name differs    => UPDATE name
		//   same            => nothing
		// FOR UPDATE locks the row so two requests can't edit it at once.
		// -----------------------------------------------------------------
		var vendor model.Vendor
		res := tx.Clauses(forUpdate).Where("external_id = ?", externalID).Limit(1).Find(&vendor)
		if res.Error != nil {
			return res.Error
		}

		switch {
		case res.RowsAffected == 0:
			vendor = model.Vendor{Name: name, ExternalID: externalID, CreatedAt: time.Now()}
			if err := tx.Create(&vendor).Error; err != nil {
				return err
			}
			result.Action = "inserted"

		case vendor.Name != name:
			result.Before = &dto.VendorBefore{Name: vendor.Name}
			if err := tx.Model(&vendor).Update("name", name).Error; err != nil {
				return err
			}
			result.Action = "updated"

		default:
			result.Action = "unchanged"
		}
		result.Vendor = vendor

		// -----------------------------------------------------------------
		// STEP 2 — STORE LINKS (store_vendors)
		// For every store: if a (store, vendor) row does not exist yet, create it
		// with picklist_creation = false. Existing rows are never changed
		// (ON CONFLICT DO NOTHING). picklist_creation is set explicitly because
		// the DB default is true.
		// -----------------------------------------------------------------
		links := tx.Exec(`
			INSERT INTO store_vendors (store_id, vendor_id, picklist_creation, created_at)
			SELECT s.id, ?, false, NOW()
			FROM stores s
			ON CONFLICT (store_id, vendor_id) DO NOTHING`, vendor.ID)
		if links.Error != nil {
			return links.Error
		}
		result.StoreLinksAdded = links.RowsAffected
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
