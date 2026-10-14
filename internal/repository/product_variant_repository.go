package repository

import (
	"context"

	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

type ProductVariantRepository struct {
	db *gorm.DB
}

func NewProductVariantRepository(db *gorm.DB) *ProductVariantRepository {
	return &ProductVariantRepository{db: db}
}

// -----------------------------------------------------------------------------
// STEP 3 — FIND THE PARENT PRODUCT by exact sku (id, sku, status).
// The handler checks status = A.
//
//	not found => nil, nil (the handler returns 422)
//
// -----------------------------------------------------------------------------
func (r *ProductVariantRepository) FindProductBySku(ctx context.Context, sku string) (*model.Product, error) {
	var product model.Product
	res := r.db.WithContext(ctx).Select("id", "sku", "status").Where("sku = ?", sku).Limit(1).Find(&product)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &product, nil
}

// -----------------------------------------------------------------------------
// STEP 4 — LINK, unique on (product_id, variant), one transaction, row locked FOR UPDATE.
//
//	not found => INSERT product_id, variant              => "inserted"
//	             status ('related') and created_at (now) come from the DB defaults:
//	             they are never sent, exactly like the CLI.
//	found     => nothing; status (related / preferred)   => "unchanged"
//	             and created_at are kept. Never update: it would overwrite 'preferred'.
//
// The same variant may already belong to OTHER products; that is allowed (not checked, same as the CLI).
// -----------------------------------------------------------------------------
func (r *ProductVariantRepository) Link(ctx context.Context, productID uint, variant string) (*dto.VariantRowReport, error) {
	var report *dto.VariantRowReport

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.ProductVariant
		res := tx.Clauses(forUpdate).Select("id", "status").
			Where("product_id = ? AND variant = ?", productID, variant).Limit(1).Find(&existing)
		if res.Error != nil {
			return res.Error
		}

		if res.RowsAffected > 0 {
			report = &dto.VariantRowReport{ID: existing.ID, Action: "unchanged", VariantStatus: existing.Status}
			return nil
		}

		// Only product_id and variant are sent; RETURNING reads back the defaulted status.
		var inserted struct {
			ID     uint
			Status string
		}
		if err := tx.Raw(`INSERT INTO product_variant (product_id, variant) VALUES (?, ?) RETURNING id, status`,
			productID, variant).Scan(&inserted).Error; err != nil {
			return err
		}
		report = &dto.VariantRowReport{ID: inserted.ID, Action: "inserted", VariantStatus: inserted.Status}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// OtherProductsWithVariant lists the OTHER products that already use this variant code
// (information only, so shared codes are easy to see). Empty slice when there are none.
func (r *ProductVariantRepository) OtherProductsWithVariant(ctx context.Context, productID uint, variant string) ([]dto.LinkedProduct, error) {
	linked := []dto.LinkedProduct{}
	err := r.db.WithContext(ctx).Raw(`
		SELECT p.id AS product_id, p.sku
		FROM product_variant v
		JOIN products p ON p.id = v.product_id
		WHERE v.variant = ? AND v.product_id <> ?
		ORDER BY p.id`, variant, productID).Scan(&linked).Error
	return linked, err
}
