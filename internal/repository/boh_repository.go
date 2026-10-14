package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

// STEP 7c (POS mode only, DISABLED): activity_logs.event value.
// const eventBohWithNoFos = "BOH_WITH_NO_FOS"

type BohRepository struct {
	db *gorm.DB
}

func NewBohRepository(db *gorm.DB) *BohRepository {
	return &BohRepository{db: db}
}

// -----------------------------------------------------------------------------
// STEP 4 — FIND THE STORE by stores.external_id (the feed's Store_ID).
// Both ids are kept: external_id comes from the feed, stores.id is used in every other table.
//
//	found     => the store (id, external_id, name)
//	not found => nil, nil (the handler returns 422)
//
// -----------------------------------------------------------------------------
func (r *BohRepository) FindStoreByExternalID(ctx context.Context, externalID int64) (*model.Store, error) {
	var store model.Store
	res := r.db.WithContext(ctx).Where("external_id = ?", externalID).Limit(1).Find(&store)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &store, nil
}

// -----------------------------------------------------------------------------
// STEP 5 — FIND THE PRODUCT by exact sku (id, sku, status).
// The handler checks status = A.
//
//	not found => nil, nil (the handler returns 422)
//
// -----------------------------------------------------------------------------
func (r *BohRepository) FindProductBySku(ctx context.Context, sku string) (*model.Product, error) {
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

// Save runs STEP 6..7 for one count inside ONE transaction.
// If any step returns an error, GORM rolls everything back, so nothing is saved.
// BOH never touches shelf quantities (product_location.qty, fos/bos_inventory, products.stock_total).
func (r *BohRepository) Save(ctx context.Context, store *model.Store, product *model.Product, qty int, date string) (*dto.BohReport, error) {
	report := &dto.BohReport{ActivityLog: "not POS mode"}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		storeQty, err := calculateStoreQty(tx, store.ID, product.ID)
		if err != nil {
			return err
		}
		report.StoreQty = storeQty

		if err := saveBoh(tx, store.ID, product.ID, qty, storeQty, date, report); err != nil {
			return err
		}

		if err := saveStoreProductQty(tx, store.ID, product.ID, qty, report); err != nil {
			return err
		}

		// STEP 7c — POS MODE ONLY (DISABLED): log when the product has no front-of-store
		// location in this store. Runs on insert AND update (same as the CLI).
		// To enable POS mode, uncomment this (and import "encoding/json"), the eventBohWithNoFos
		// constant, the POS variant in calculateStoreQty, and set appMode "pos" in the handler.
		/*
			var hasFos bool
			if err := tx.Raw(`
				SELECT EXISTS (
					SELECT 1 FROM product_location pl
					JOIN store_location sl ON sl.id = pl.location_id
					WHERE pl.product_id = ? AND sl.store_id = ? AND sl.is_back = false
				)`, product.ID, store.ID).Scan(&hasFos).Error; err != nil {
				return err
			}

			if hasFos {
				report.ActivityLog = "product has FOS location"
			} else {
				metadata, err := json.Marshal(map[string]any{
					"productCount": 1,
					"products":     []map[string]any{{"sku": product.Sku, "bohId": report.Boh.ID}},
				})
				if err != nil {
					return err
				}
				// activity_logs.id is GENERATED ALWAYS, so it is never sent. user_id NULL = feed import.
				var logID uint
				if err := tx.Raw(`
					INSERT INTO activity_logs (user_id, store_id, event, created_at, metadata)
					VALUES (NULL, ?, ?, ?, ?) RETURNING id`,
					store.ID, eventBohWithNoFos, time.Now(), string(metadata)).Scan(&logID).Error; err != nil {
					return err
				}
				report.ActivityLog = map[string]uint{"id": logID}
			}
		*/
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// -----------------------------------------------------------------------------
// STEP 6 — store_qty: what the system holds in this store BEFORE this count.
// Matched on stores.id (the CLI compared external_id here, which was only right when id = external_id).
//
//	default mode (ACTIVE)   => SUM(product_location.qty) over ALL locations (back + front), 0 if none
//	POS mode     (DISABLED) => store_products.total_qty, read before STEP 7b changes it, 0 if no row
//
// -----------------------------------------------------------------------------
func calculateStoreQty(tx *gorm.DB, storeID, productID uint) (int, error) {
	var storeQty int
	err := tx.Raw(`
		SELECT COALESCE(SUM(pl.qty), 0)
		FROM product_location pl
		JOIN store_location sl ON sl.id = pl.location_id
		WHERE pl.product_id = ? AND sl.store_id = ?`, productID, storeID).Scan(&storeQty).Error

	// POS mode (DISABLED): replace the query above with
	//
	//	err := tx.Raw(`SELECT COALESCE(SUM(total_qty), 0) FROM store_products
	//		WHERE store_id = ? AND product_id = ?`, storeID, productID).Scan(&storeQty).Error

	return storeQty, err
}

// -----------------------------------------------------------------------------
// STEP 7a — BOH ROW, unique on (store_id, product_id, date), row locked FOR UPDATE.
//
//	not found   => INSERT qty, store_qty, date, created_at        => "inserted"
//	qty differs => UPDATE qty only (store_qty, created_at keep    => "updated" + before
//	               their first values, same as the CLI)
//	same qty    => nothing                                        => "unchanged"
//
// -----------------------------------------------------------------------------
func saveBoh(tx *gorm.DB, storeID, productID uint, qty, storeQty int, date string, report *dto.BohReport) error {
	var existing model.Boh
	res := tx.Clauses(forUpdate).Select("id", "qty").
		Where("store_id = ? AND product_id = ? AND date = ?", storeID, productID, date).Limit(1).Find(&existing)

	switch {
	case res.Error != nil:
		return res.Error

	case res.RowsAffected == 0:
		boh := model.Boh{StoreID: storeID, ProductID: productID, Qty: qty, StoreQty: storeQty, Date: date, CreatedAt: time.Now()}
		if err := tx.Create(&boh).Error; err != nil {
			return err
		}
		report.Boh = dto.BohRowReport{ID: boh.ID, Action: "inserted"}

	case existing.Qty != qty:
		if err := tx.Model(&model.Boh{}).Where("id = ?", existing.ID).Update("qty", qty).Error; err != nil {
			return err
		}
		report.Boh = dto.BohRowReport{ID: existing.ID, Action: "updated", Before: &dto.BohBefore{Qty: existing.Qty}}

	default:
		report.Boh = dto.BohRowReport{ID: existing.ID, Action: "unchanged"}
	}
	return nil
}

// -----------------------------------------------------------------------------
// STEP 7b — STORE_PRODUCTS ROW, unique on (store_id, product_id), row locked FOR UPDATE.
//
//	not found         => INSERT store_id, product_id, total_qty (other columns = DB defaults) => "inserted"
//	total_qty differs => UPDATE total_qty                                                    => "updated"
//	same              => nothing                                                             => "unchanged"
//
// The date is NOT compared: an older date still overwrites total_qty (last request wins, same as the CLI).
// -----------------------------------------------------------------------------
func saveStoreProductQty(tx *gorm.DB, storeID, productID uint, qty int, report *dto.BohReport) error {
	var existing model.StoreProduct
	res := tx.Clauses(forUpdate).Select("id", "total_qty").
		Where("store_id = ? AND product_id = ?", storeID, productID).Limit(1).Find(&existing)

	switch {
	case res.Error != nil:
		return res.Error

	case res.RowsAffected == 0:
		row := model.StoreProduct{StoreID: storeID, ProductID: productID, TotalQty: qty}
		if err := tx.Select("StoreID", "ProductID", "TotalQty").Create(&row).Error; err != nil {
			return err
		}
		report.StoreProduct = dto.StoreProductReport{Action: "inserted"}

	case existing.TotalQty != qty:
		if err := tx.Model(&model.StoreProduct{}).Where("id = ?", existing.ID).Update("total_qty", qty).Error; err != nil {
			return err
		}
		before := existing.TotalQty
		report.StoreProduct = dto.StoreProductReport{Action: "updated", Before: &before}

	default:
		current := existing.TotalQty
		report.StoreProduct = dto.StoreProductReport{Action: "unchanged", Before: &current}
	}
	return nil
}
