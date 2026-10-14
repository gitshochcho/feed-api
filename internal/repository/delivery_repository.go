package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

const (
	deliveryStatusPlanned = "planned"
	// STEP 7 (DISABLED): values written when a delivery is finished on feed.
	// deliveryStatusFinished = "finished"
	// processTypeOnFeed      = "processedOnFeed"
)

type DeliveryRepository struct {
	db *gorm.DB
}

func NewDeliveryRepository(db *gorm.DB) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

// -----------------------------------------------------------------------------
// STEP 4 — FIND THE STORE by stores.external_id (the feed's Store_ID).
//
//	found     => the store (id, external_id); stores.id is used in every other table
//	not found => nil, nil (the handler logs it and returns 422)
//
// -----------------------------------------------------------------------------
func (r *DeliveryRepository) FindStoreByExternalID(ctx context.Context, externalID int64) (*model.Store, error) {
	var store model.Store
	res := r.db.WithContext(ctx).Select("id", "external_id").Where("external_id = ?", externalID).Limit(1).Find(&store)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &store, nil
}

// -----------------------------------------------------------------------------
// STEP 5 — FIND THE PRODUCT by exact sku (id, sku, status, inner_pack).
// The handler checks status = A.
//
//	not found => nil, nil (the handler logs it and returns 422)
//
// -----------------------------------------------------------------------------
func (r *DeliveryRepository) FindProductBySku(ctx context.Context, sku string) (*model.Product, error) {
	var product model.Product
	res := r.db.WithContext(ctx).Select("id", "sku", "status", "inner_pack").Where("sku = ?", sku).Limit(1).Find(&product)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &product, nil
}

// LogActivity writes one activity_logs row for a skipped feed row (same as the CLI event listener).
// It runs OUTSIDE any transaction, so the log survives when the row itself is rejected.
// user_id is NULL (feed import, no user); id is GENERATED ALWAYS and never sent.
func (r *DeliveryRepository) LogActivity(ctx context.Context, event string, storeID *uint, metadata any) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	entry := model.ActivityLog{StoreID: storeID, Event: event, CreatedAt: time.Now(), Metadata: string(data)}
	return r.db.WithContext(ctx).Select("UserID", "StoreID", "Event", "CreatedAt", "Metadata").Create(&entry).Error
}

// Save runs STEP 6a..6c for one row inside ONE transaction. No stock moves here.
// If the delivery has already started, nothing is written and report.Blocked is true
// (the handler then logs it and returns 409).
func (r *DeliveryRepository) Save(ctx context.Context, store *model.Store, product *model.Product, row dto.DeliveryRow) (*dto.DeliveryReport, error) {
	report := &dto.DeliveryReport{}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deliveryID, err := saveDelivery(tx, store.ID, row, report)
		if err != nil || report.Blocked {
			return err
		}
		unitID, err := saveDeliveryUnit(tx, deliveryID, row.DeliveryUnitCode, report)
		if err != nil {
			return err
		}
		return saveDeliveryProduct(tx, unitID, product.ID, row, report)
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// -----------------------------------------------------------------------------
// STEP 6a — DELIVERY, unique on (asn, store_id = INTERNAL id), row locked FOR UPDATE.
//
//	not found           => INSERT store_id, asn, eta, from, status 'planned'      => "inserted"
//	                       (timestamps, prepared_by, process type, picklist group stay NULL)
//	found, planned      => UPDATE eta and from if either changed                   => "updated" / "unchanged"
//	found, other status => write nothing, report.Blocked = true                    => handler: log + 409
//	                       (the CLI would reset it to 'planned' on a new pallet; the API blocks it)
//
// -----------------------------------------------------------------------------
func saveDelivery(tx *gorm.DB, storeID uint, row dto.DeliveryRow, report *dto.DeliveryReport) (uint, error) {
	var existing model.Delivery
	res := tx.Clauses(forUpdate).Where("store_id = ? AND asn = ?", storeID, row.Asn).Limit(1).Find(&existing)
	if res.Error != nil {
		return 0, res.Error
	}

	if res.RowsAffected == 0 {
		delivery := model.Delivery{StoreID: storeID, Asn: row.Asn, Eta: row.Eta, From: row.From, Status: deliveryStatusPlanned}
		if err := tx.Select("StoreID", "Asn", "Eta", "From", "Status").Create(&delivery).Error; err != nil {
			return 0, err
		}
		report.Delivery = dto.DeliveryRowReport{ID: delivery.ID, Asn: row.Asn, Action: "inserted", Changes: map[string]dto.FieldChange{}}
		return delivery.ID, nil
	}

	if existing.Status != deliveryStatusPlanned {
		report.Blocked, report.BlockedDeliveryID, report.BlockedStatus = true, existing.ID, existing.Status
		return existing.ID, nil
	}

	changes := map[string]dto.FieldChange{}
	if !existing.Eta.Equal(row.Eta) {
		changes["eta"] = dto.FieldChange{From: isoUTC(existing.Eta), To: isoUTC(row.Eta)}
	}
	if existing.From != row.From {
		changes["from"] = dto.FieldChange{From: existing.From, To: row.From}
	}

	action := "unchanged"
	if len(changes) > 0 {
		if err := tx.Model(&model.Delivery{}).Where("id = ?", existing.ID).
			Updates(map[string]any{"eta": row.Eta, "from": row.From}).Error; err != nil {
			return 0, err
		}
		action = "updated"
	}
	report.Delivery = dto.DeliveryRowReport{ID: existing.ID, Asn: row.Asn, Action: action, Changes: changes}
	return existing.ID, nil
}

// -----------------------------------------------------------------------------
// STEP 6b — DELIVERY UNIT (pallet), unique on (delivery_id, delivery_unit_code), row locked.
//
//	not found => INSERT delivery_id, delivery_unit_code => "inserted"
//	             (processing_start_at, processing_end_at, fos_allocation_percentage stay NULL)
//	found     => nothing                                => "unchanged"
//
// -----------------------------------------------------------------------------
func saveDeliveryUnit(tx *gorm.DB, deliveryID uint, code string, report *dto.DeliveryReport) (uint, error) {
	var existing model.DeliveryUnit
	res := tx.Clauses(forUpdate).Where("delivery_id = ? AND delivery_unit_code = ?", deliveryID, code).Limit(1).Find(&existing)
	if res.Error != nil {
		return 0, res.Error
	}

	if res.RowsAffected > 0 {
		report.DeliveryUnit = dto.DeliveryUnitReport{ID: existing.ID, Code: code, Action: "unchanged"}
		return existing.ID, nil
	}

	unit := model.DeliveryUnit{DeliveryID: deliveryID, DeliveryUnitCode: code}
	if err := tx.Select("DeliveryID", "DeliveryUnitCode").Create(&unit).Error; err != nil {
		return 0, err
	}
	report.DeliveryUnit = dto.DeliveryUnitReport{ID: unit.ID, Code: code, Action: "inserted"}
	return unit.ID, nil
}

// -----------------------------------------------------------------------------
// STEP 6c — DELIVERY PRODUCT, unique on (delivery_unit_id, product_id), row locked.
//
//	not found      => INSERT expected_cases, expected_units, damaged_cases, description => "inserted"
//	                  (delivered_cases, delivered_units, verified_by stay NULL)
//	any of 4 differ => UPDATE all 4 columns: new values REPLACE old ones (not added)  => "updated"
//	same            => nothing                                                         => "unchanged"
//
// -----------------------------------------------------------------------------
func saveDeliveryProduct(tx *gorm.DB, unitID, productID uint, row dto.DeliveryRow, report *dto.DeliveryReport) error {
	var existing model.DeliveryProduct
	res := tx.Clauses(forUpdate).Where("delivery_unit_id = ? AND product_id = ?", unitID, productID).Limit(1).Find(&existing)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		dp := model.DeliveryProduct{
			DeliveryUnitID: unitID, ProductID: productID,
			ExpectedCases: row.ExpectedCases, ExpectedUnits: row.ExpectedUnits,
			DamagedCases: &row.DamagedCases, Description: &row.Description,
		}
		if err := tx.Select("DeliveryUnitID", "ProductID", "ExpectedCases", "ExpectedUnits", "DamagedCases", "Description").
			Create(&dp).Error; err != nil {
			return err
		}
		report.DeliveryProduct = dto.DeliveryProductRowReport{ID: dp.ID, Action: "inserted", Changes: map[string]dto.FieldChange{}}
		return nil
	}

	// Compare as text, like PHP's (string) cast (NULL => "").
	compare := []struct {
		column   string
		old, new any
	}{
		{"expected_cases", existing.ExpectedCases, row.ExpectedCases},
		{"expected_units", existing.ExpectedUnits, row.ExpectedUnits},
		{"damaged_cases", intPtrValue(existing.DamagedCases), row.DamagedCases},
		{"description", strPtrValue(existing.Description), row.Description},
	}
	changes := map[string]dto.FieldChange{}
	for _, c := range compare {
		if valueText(c.old) != valueText(c.new) {
			changes[c.column] = dto.FieldChange{From: c.old, To: c.new}
		}
	}

	action := "unchanged"
	if len(changes) > 0 {
		if err := tx.Model(&model.DeliveryProduct{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"expected_cases": row.ExpectedCases,
			"expected_units": row.ExpectedUnits,
			"damaged_cases":  row.DamagedCases,
			"description":    row.Description,
		}).Error; err != nil {
			return err
		}
		action = "updated"
	}
	report.DeliveryProduct = dto.DeliveryProductRowReport{ID: existing.ID, Action: action, Changes: changes}
	return nil
}

// =============================================================================
// STEP 7 — POST-FEED PROCESSING (DISABLED)
//
// Business decision: off, with no env flag. With one row per request it would finish
// the delivery after its FIRST row, so every later row for the same ASN would get 409.
// To enable it: uncomment PostProcess below and the two constants at the top, then call it
// from the handler after Save (it runs in a second transaction):
//
//	postProcessing, err := h.repo.PostProcess(ctx, store.ID, product, deliveryID, unitID, deliveryProductID)
//
// (Save would then also need to return the three ids.)
//
//	7a put stock away (only if: not delivered yet AND a back-of-store location AND a store_products row exist):
//	   cases/units = delivered value if set, else expected; totalUnits = cases*inner_pack + units
//	   first BOS product_location (lowest id): case += cases, units += units, qty = case*inner_pack + units
//	   delivery_product_assignations row with a snapshot of the location (only when cases or units != 0)
//	   store_products: bos_inventory += totalUnits, total_qty = fos_inventory + bos_inventory
//	   delivery_products: delivered_cases = cases, delivered_units = units
//	7b close the pallet if processing_start_at IS NULL (start/end times, fos_allocation_percentage 0)
//	7c finish the delivery if planned and preparation_start_at IS NULL (status finished, processedOnFeed)
// =============================================================================
/*
func (r *DeliveryRepository) PostProcess(ctx context.Context, storeID uint, product *model.Product, deliveryID, unitID, deliveryProductID uint) (map[string]any, error) {
	report := map[string]any{"stock": nil, "deliveryUnitClosed": false, "deliveryFinished": false}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		startAt := time.Now()

		// 7a — current values, all rows locked.
		var dp struct {
			ID                            uint
			ExpectedCases, ExpectedUnits  int
			DeliveredCases, DeliveredUnits *int
		}
		if err := tx.Raw(`SELECT id, expected_cases, expected_units, delivered_cases, delivered_units
			FROM delivery_products WHERE id = ? FOR UPDATE`, deliveryProductID).Scan(&dp).Error; err != nil {
			return err
		}

		var sp struct {
			ID                         uint
			FosInventory, BosInventory int
		}
		spRes := tx.Raw(`SELECT id, fos_inventory, bos_inventory FROM store_products
			WHERE store_id = ? AND product_id = ? FOR UPDATE`, storeID, product.ID).Scan(&sp)
		if spRes.Error != nil {
			return spRes.Error
		}

		// First back-of-store location of this product in this store (lowest id). "case" is reserved.
		var bos struct {
			ID, LocationID                 uint
			Case, Units                    *int
			Name, FixtureType, Side        *string
			IsBack                         bool
			Aisle, Bay, Shelf, Position    *int
		}
		bosRes := tx.Raw(`SELECT pl.id, pl."case", pl.units, sl.id AS location_id, sl.name, sl.is_back,
				sl.fixture_type, sl.aisle, sl.side, sl.bay, sl.shelf, sl.position
			FROM product_location pl JOIN store_location sl ON sl.id = pl.location_id
			WHERE pl.product_id = ? AND sl.store_id = ? AND sl.is_back = true
			ORDER BY pl.id LIMIT 1 FOR UPDATE OF pl`, product.ID, storeID).Scan(&bos)
		if bosRes.Error != nil {
			return bosRes.Error
		}

		switch {
		case dp.DeliveredCases != nil && dp.DeliveredUnits != nil:
			report["stock"] = "skipped: already delivered"
		case bosRes.RowsAffected == 0:
			report["stock"] = "skipped: no back-of-store location in this store"
		case spRes.RowsAffected == 0:
			report["stock"] = "skipped: no store_products row in this store"
		default:
			cases, units := dp.ExpectedCases, dp.ExpectedUnits
			if dp.DeliveredCases != nil {
				cases = *dp.DeliveredCases
			}
			if dp.DeliveredUnits != nil {
				units = *dp.DeliveredUnits
			}
			totalUnits := cases*product.InnerPack + units

			// Nothing to place when both are 0 (same as the CLI).
			if cases != 0 || units != 0 {
				newCase, newUnits := cases, units
				if bos.Case != nil {
					newCase += *bos.Case
				}
				if bos.Units != nil {
					newUnits += *bos.Units
				}
				if err := tx.Exec(`UPDATE product_location SET "case" = ?, units = ?, qty = ? WHERE id = ?`,
					newCase, newUnits, newCase*product.InnerPack+newUnits, bos.ID).Error; err != nil {
					return err
				}

				snapshot, err := json.Marshal(map[string]any{
					"id": bos.LocationID, "name": bos.Name, "isBack": bos.IsBack, "fixtureType": bos.FixtureType,
					"aisle": bos.Aisle, "side": bos.Side, "bay": bos.Bay, "shelf": bos.Shelf, "position": bos.Position,
				})
				if err != nil {
					return err
				}
				if err := tx.Exec(`INSERT INTO delivery_product_assignations
					(delivery_product_id, product_location_id, cases, units, user_id, created_at, metadata)
					VALUES (?, ?, ?, ?, NULL, ?, ?)`, dp.ID, bos.ID, cases, units, time.Now(), string(snapshot)).Error; err != nil {
					return err
				}
			}

			// Store totals: back of store grows, total = front + back.
			newBos := sp.BosInventory + totalUnits
			if err := tx.Exec(`UPDATE store_products SET bos_inventory = ?, total_qty = ? WHERE id = ?`,
				newBos, sp.FosInventory+newBos, sp.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`UPDATE delivery_products SET delivered_cases = ?, delivered_units = ? WHERE id = ?`,
				cases, units, dp.ID).Error; err != nil {
				return err
			}
			report["stock"] = map[string]any{"productLocationId": bos.ID, "cases": cases, "units": units, "totalUnits": totalUnits}
		}

		endAt := time.Now()

		// 7b — close the pallet (only if not started yet).
		closed := tx.Exec(`UPDATE delivery_units SET processing_start_at = ?, processing_end_at = ?, fos_allocation_percentage = 0
			WHERE id = ? AND processing_start_at IS NULL`, startAt, endAt, unitID)
		if closed.Error != nil {
			return closed.Error
		}
		report["deliveryUnitClosed"] = closed.RowsAffected > 0

		// 7c — finish the delivery (only if planned and not prepared yet).
		finished := tx.Exec(`UPDATE deliveries SET preparation_start_at = ?, received_at = ?, preparation_end_at = ?,
				processing_end_at = ?, status = ?, delivery_process_type = ?
			WHERE id = ? AND preparation_start_at IS NULL AND status = ?`,
			startAt, endAt, endAt, endAt, deliveryStatusFinished, processTypeOnFeed, deliveryID, deliveryStatusPlanned)
		if finished.Error != nil {
			return finished.Error
		}
		report["deliveryFinished"] = finished.RowsAffected > 0
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}
*/

// --- small helpers -----------------------------------------------------------

// isoUTC formats a time like Laravel's toIso8601String() in UTC: 2026-10-20T08:00:00+00:00.
func isoUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05-07:00")
}

// intPtrValue returns nil for NULL, otherwise the number.
func intPtrValue(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
