package repository

import (
	"context"

	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

type ProductUpcRepository struct {
	db *gorm.DB
}

func NewProductUpcRepository(db *gorm.DB) *ProductUpcRepository {
	return &ProductUpcRepository{db: db}
}

// -----------------------------------------------------------------------------
// STEP 4 — FIND THE PRODUCT (exact sku match)
// UPCs never create products. Product status is NOT checked: inactive products also get UPCs.
//
//	found     => the product (id + sku)
//	not found => nil, nil (the handler returns 422)
//
// -----------------------------------------------------------------------------
func (r *ProductUpcRepository) FindProductBySku(ctx context.Context, sku string) (*model.Product, error) {
	var product model.Product
	res := r.db.WithContext(ctx).Select("id", "sku").Where("sku = ?", sku).Limit(1).Find(&product)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &product, nil
}

// Save runs STEP 5..6 for one UPC inside ONE transaction.
// If any step returns an error, GORM rolls everything back. Nothing is ever deleted.
//
// STEP 5 — find the UPC (upc is UNIQUE), row locked FOR UPDATE.
// STEP 6 — insert, move or keep:
//
//	6a not found                 => INSERT (product_id, upc)                  => "inserted"
//	6b found, different product  => UPDATE product_id (the old product loses it) => "updated" + before
//	6c found, same product       => nothing                                   => "unchanged"
//
// The upc value itself is never changed.
func (r *ProductUpcRepository) Save(ctx context.Context, upc string, product *model.Product) (*dto.ProductUpcReport, error) {
	var report *dto.ProductUpcReport

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// STEP 5 — find the UPC and lock it.
		var existing model.ProductUpc
		res := tx.Clauses(forUpdate).Where("upc = ?", upc).Limit(1).Find(&existing)
		if res.Error != nil {
			return res.Error
		}

		switch {
		case res.RowsAffected == 0:
			// STEP 6a — not found => INSERT (GORM fills newUpc.ID).
			newUpc := model.ProductUpc{ProductID: product.ID, Upc: upc}
			if err := tx.Create(&newUpc).Error; err != nil {
				return err
			}
			report = &dto.ProductUpcReport{Action: "inserted", UpcID: newUpc.ID}

		case existing.ProductID != product.ID:
			// STEP 6b — found on a different product => MOVE it. Remember the old owner first.
			var oldSku string
			// product_id is a foreign key, so the old product always exists.
			if err := tx.Model(&model.Product{}).Select("sku").Where("id = ?", existing.ProductID).
				Row().Scan(&oldSku); err != nil {
				return err
			}
			if err := tx.Model(&model.ProductUpc{}).Where("id = ?", existing.ID).
				Update("product_id", product.ID).Error; err != nil {
				return err
			}
			report = &dto.ProductUpcReport{
				Action: "updated",
				UpcID:  existing.ID,
				Before: &dto.UpcBefore{ProductID: existing.ProductID, Sku: oldSku},
			}

		default:
			// STEP 6c — found on the same product => nothing to do.
			report = &dto.ProductUpcReport{Action: "unchanged", UpcID: existing.ID}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}
