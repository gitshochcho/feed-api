package repository

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
)

const (
	// STEP 8b: name of the shared back-of-store location in every store.
	backRoomName = "Back Room"
	// STEP 8b: prefix of the per-product front-of-store location. Full name = prefix + product id.
	virtualFosPrefix = "Virtual FoS - "
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// -----------------------------------------------------------------------------
// STEP 5 — FIND THE CATEGORY
// All 5 keys must match exactly ("" matches ""). Products never create categories.
//
//	found     => the category (id + department_id are used later)
//	not found => nil, nil (the handler returns 422)
//
// -----------------------------------------------------------------------------
func (r *ProductRepository) FindCategory(ctx context.Context, levels [5]string) (*model.Category, error) {
	var category model.Category
	res := r.db.WithContext(ctx).Where(map[string]any{
		"level_1_key": levels[0],
		"level_2_key": levels[1],
		"level_3_key": levels[2],
		"level_4_key": levels[3],
		"level_5_key": levels[4],
	}).Limit(1).Find(&category)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &category, nil
}

// Import runs STEP 6..8 for one product inside ONE transaction.
// If any step returns an error, GORM rolls everything back, so nothing is saved.
// STEP 9 (empty-picklist cleanup) is NOT part of this API: it is a store-wide job.
// moq is the store_products.moq value from the request (the handler defaults it to 1).
func (r *ProductRepository) Import(ctx context.Context, product model.Product, category *model.Category, moq float64) (*dto.ProductImportReport, error) {
	report := &dto.ProductImportReport{}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		brandID, err := saveBrand(tx, product.Brand, now, report)
		if err != nil {
			return err
		}
		product.BrandID = brandID

		if err := saveProduct(tx, &product, now, report); err != nil {
			return err
		}

		// status I => STEP 8a only (8b is skipped). status A => STEP 8b only.
		if product.Status == "I" {
			return cleanupInactive(tx, product.ID, report)
		}
		return ensureLocations(tx, product, category, moq, now, report)
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// -----------------------------------------------------------------------------
// STEP 6 — BRAND
// brandKey = brand in UPPERCASE with everything except A-Z / 0-9 removed ("Wall's" => WALLS).
//
//	brand empty OR brandKey empty => brand_id NULL, no brand row written
//	brandKey not found            => INSERT (name, external_id, created_at)
//	found, name differs           => UPDATE name (the last product sent wins)
//	found, same name              => nothing
//
// product_brands.name is UNIQUE too: if another row already has this name,
// the INSERT/UPDATE fails with ErrDuplicatedKey and the whole request rolls back.
// -----------------------------------------------------------------------------
func saveBrand(tx *gorm.DB, brand string, now time.Time, report *dto.ProductImportReport) (*uint, error) {
	brandKey := brandKeyOf(brand)
	if brand == "" || brandKey == "" {
		return nil, nil
	}

	var existing model.ProductBrand
	res := tx.Clauses(forUpdate).Where("external_id = ?", brandKey).Limit(1).Find(&existing)

	switch {
	case res.Error != nil:
		return nil, res.Error

	case res.RowsAffected == 0:
		// Not found => INSERT.
		newBrand := model.ProductBrand{Name: brand, ExternalID: brandKey, CreatedAt: now}
		if err := tx.Create(&newBrand).Error; err != nil {
			return nil, err
		}
		report.Brand = &dto.BrandReport{ID: newBrand.ID, ExternalID: brandKey, Action: "inserted"}
		return &newBrand.ID, nil

	case existing.Name != brand:
		// Found but the spelling differs => UPDATE name, and remember the old one.
		if err := tx.Model(&model.ProductBrand{}).Where("id = ?", existing.ID).Update("name", brand).Error; err != nil {
			return nil, err
		}
		before := existing.Name
		report.Brand = &dto.BrandReport{ID: existing.ID, ExternalID: brandKey, Action: "updated", Before: &before}

	default:
		// Found and identical => nothing to do.
		report.Brand = &dto.BrandReport{ID: existing.ID, ExternalID: brandKey, Action: "unchanged"}
	}
	return &existing.ID, nil
}

// brandKeyOf copies PHP: preg_replace('/[^A-Z0-9]+/', ”, strtoupper($brand)).
// strtoupper only changes ASCII a-z, so this works byte by byte.
func brandKeyOf(brand string) string {
	key := make([]byte, 0, len(brand))
	for i := 0; i < len(brand); i++ {
		b := brand[i]
		if b >= 'a' && b <= 'z' {
			b -= 'a' - 'A'
		}
		if (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') {
			key = append(key, b)
		}
	}
	return string(key)
}

// -----------------------------------------------------------------------------
// STEP 7 — PRODUCT (matched on sku, row locked FOR UPDATE)
//
//	not found => INSERT sku + mapped columns + fixed values
//	found     => UPDATE mapped columns + fixed values
//
// Fixed values are written on update too (same as the CLI): stock/min/max/promo
// reset to 0, flags false, created_at = now.
// The response lists only the mapped columns whose values really changed.
// -----------------------------------------------------------------------------
func saveProduct(tx *gorm.DB, p *model.Product, now time.Time, report *dto.ProductImportReport) error {
	// Fixed values (insert and update).
	p.StockTotal, p.StockFs, p.StockBs = 0, "", ""
	p.Min, p.Max, p.PromoPrice = 0, 0, 0
	p.IsVmi, p.IsFifo, p.IsDeal = false, false, false
	p.CreatedAt = now

	// Read the existing row as a map, so NULL columns and DB types are seen as they are.
	existing := map[string]any{}
	res := tx.Table("products").Clauses(forUpdate).Where("sku = ?", p.Sku).Limit(1).Find(&existing)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		// Not found => INSERT (GORM fills p.ID).
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		report.Product = dto.ProductReport{ID: p.ID, Sku: p.Sku, Action: "inserted", Changes: map[string]dto.FieldChange{}}
		return nil
	}

	// Found => compare each mapped column with the DB value (response only; all are written).
	p.ID = uint(toInt64(existing["id"]))
	changes := map[string]dto.FieldChange{}
	columns := []string{}
	for _, col := range mappedColumns(p) {
		old := dbValue(existing[col.name])
		if valueChanged(old, col.value) {
			changes[col.name] = dto.FieldChange{From: old, To: col.value}
		}
		columns = append(columns, col.name)
	}
	columns = append(columns, "stock_total", "stock_fs", "stock_bs", "min", "max", "promo_price",
		"is_vmi", "is_fifo", "is_deal", "created_at")

	// UPDATE with an explicit column list, so "", 0, false and NULL are written too.
	if err := tx.Model(&model.Product{}).Where("id = ?", p.ID).Select(columns).Updates(p).Error; err != nil {
		return err
	}

	action := "unchanged"
	if len(changes) > 0 {
		action = "updated"
	}
	report.Product = dto.ProductReport{ID: p.ID, Sku: p.Sku, Action: action, Changes: changes}
	return nil
}

type columnValue struct {
	name  string
	value any
}

// mappedColumns are the products columns that come from the feed (spec 1.3), in spec order.
func mappedColumns(p *model.Product) []columnValue {
	return []columnValue{
		{"category_id", p.CategoryID},
		{"name", p.Name},
		{"brand", p.Brand},
		{"brand_id", uintPtrValue(p.BrandID)},
		{"status", p.Status},
		{"vendor_key", p.VendorKey},
		{"uom_buy", p.UomBuy},
		{"buy_qty", p.BuyQty},
		{"uom_sell", p.UomSell},
		{"inner_pack", p.InnerPack},
		{"master_pack", p.MasterPack},
		{"image", p.Image},
		{"primary_upc", p.PrimaryUpc},
		{"retail_price", p.RetailPrice},
		{"unit_landed_cost", p.UnitLandedCost},
		{"sku_type", p.SkuType},
		{"parent_sku", p.ParentSku},
		{"qty_per_parent", p.QtyPerParent},
		{"reference_sku", p.ReferenceSku},
		{"shelf_days", p.ShelfDays},
	}
}

// -----------------------------------------------------------------------------
// STEP 8a — INACTIVE PRODUCT (status = I)
//
//	8a-1 DISABLED: delete product_location rows in front-of-store locations (is_back = false), all stores
//	8a-2 DISABLED: delete those front-of-store locations if they no longer hold any product
//	8a-3 ACTIVE:   delete picklist_product_assignations, then picklist_products of this product
//	8a-4:          BOS rows, FOS rows (while disabled) and store_products are kept
//	8a-5:          stop here, STEP 8b does not run
//
// -----------------------------------------------------------------------------
func cleanupInactive(tx *gorm.DB, productID uint, report *dto.ProductImportReport) error {
	cleanup := &dto.InactiveCleanupReport{FosRemovalEnabled: false}

	// STEP 8a-1 and 8a-2 — FRONT-OF-STORE REMOVAL (DISABLED)
	// Business decision: for now an inactive product keeps ALL its product_location
	// rows and no store_location is deleted. To enable the CLI behaviour, uncomment
	// the block below. Note 8a-1 also removes real aisle/planogram locations.
	/*
		cleanup.FosRemovalEnabled = true

		var fosLocationIDs []uint
		if err := tx.Table("product_location AS pl").
			Joins("JOIN store_location sl ON sl.id = pl.location_id").
			Where("pl.product_id = ? AND sl.is_back = false", productID).
			Distinct().Pluck("pl.location_id", &fosLocationIDs).Error; err != nil {
			return err
		}

		// 8a-1: product_location rows of this product in front-of-store locations.
		removed := tx.Exec(`DELETE FROM product_location pl USING store_location sl
			WHERE sl.id = pl.location_id AND pl.product_id = ? AND sl.is_back = false`, productID)
		if removed.Error != nil {
			return removed.Error
		}
		cleanup.FosProductLocationsRemoved = removed.RowsAffected

		// 8a-2: those front-of-store locations, only if they are now empty.
		if len(fosLocationIDs) > 0 {
			emptied := tx.Exec(`DELETE FROM store_location sl
				WHERE sl.id IN ? AND sl.is_back = false
				AND NOT EXISTS (SELECT 1 FROM product_location pl2 WHERE pl2.location_id = sl.id)`, fosLocationIDs)
			if emptied.Error != nil {
				return emptied.Error
			}
			cleanup.EmptyFosLocationsRemoved = emptied.RowsAffected
		}
	*/

	// STEP 8a-3 — PICKLIST CLEANUP (ENABLED)
	// Assignations first: they reference picklist_products.
	var picklistProductIDs []uint
	if err := tx.Table("picklist_products").Where("product_id = ?", productID).Pluck("id", &picklistProductIDs).Error; err != nil {
		return err
	}
	if len(picklistProductIDs) > 0 {
		if err := tx.Exec("DELETE FROM picklist_product_assignations WHERE picklist_product_id IN ?", picklistProductIDs).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM picklist_products WHERE id IN ?", picklistProductIDs).Error; err != nil {
			return err
		}
	}
	cleanup.PicklistProductsRemoved = int64(len(picklistProductIDs))

	report.InactiveCleanup = cleanup
	return nil
}

// -----------------------------------------------------------------------------
// STEP 8b — ACTIVE PRODUCT (status = A): BOS + FOS in every store
//
//	8b-1/2: hasBos / hasFos = product already in a back / front location of this store
//	8b-3:   both true             => nothing to do for this store
//	8b-4:   find or create the store's "Back Room" (is_back = true)
//	8b-5:   hasBos false          => upsert store_products + product_location in the Back Room
//	8b-6:   hasFos false          => find or create "Virtual FoS - {id}" + upsert store_products + product_location
//
// store_products.moq = moq from the request (default 1).
// Existing product_location rows are never updated (ON CONFLICT DO NOTHING).
// -----------------------------------------------------------------------------
func ensureLocations(tx *gorm.DB, p model.Product, category *model.Category, moq float64, now time.Time, report *dto.ProductImportReport) error {
	locations := &dto.LocationsReport{}

	var storeIDs []uint
	if err := tx.Table("stores").Order("id").Pluck("id", &storeIDs).Error; err != nil {
		return err
	}

	for _, storeID := range storeIDs {
		locations.StoresChecked++

		// STEP 8b-1 + 8b-2 — hasBos / hasFos in one query.
		var has struct{ HasBos, HasFos bool }
		if err := tx.Raw(`
			SELECT COALESCE(bool_or(sl.is_back), false)     AS has_bos,
			       COALESCE(bool_or(NOT sl.is_back), false) AS has_fos
			FROM product_location pl
			JOIN store_location sl ON sl.id = pl.location_id
			WHERE pl.product_id = ? AND sl.store_id = ?`, p.ID, storeID).Scan(&has).Error; err != nil {
			return err
		}

		// STEP 8b-3 — both present => nothing to do for this store.
		if has.HasBos && has.HasFos {
			locations.StoresAlreadyComplete++
			continue
		}

		// STEP 8b-4 — find or create the store's "Back Room" (only these 4 columns, like Laravel).
		roomName := backRoomName
		backRoomID, created, err := findOrCreateLocation(tx,
			model.StoreLocation{StoreID: storeID, IsBack: true, Name: &roomName, CreatedAt: now},
			"StoreID", "IsBack", "Name", "CreatedAt")
		if err != nil {
			return err
		}
		if created {
			locations.BackRoomsCreated++
		}

		// STEP 8b-5 — BOS missing => store_products + product in the Back Room.
		if !has.HasBos {
			if err := upsertStoreProduct(tx, storeID, p.ID, moq); err != nil {
				return err
			}
			added, err := insertProductLocation(tx, backRoomID, p, now)
			if err != nil {
				return err
			}
			locations.BosAdded += added
		}

		// STEP 8b-6 — FOS missing => "Virtual FoS - {id}" + store_products + product_location.
		// department_id = product -> category -> department.
		if !has.HasFos {
			fosName := virtualFosPrefix + strconv.FormatUint(uint64(p.ID), 10)
			fixtureType := "Virtual"
			fosID, created, err := findOrCreateLocation(tx,
				model.StoreLocation{StoreID: storeID, IsBack: false, FixtureType: &fixtureType, Name: &fosName,
					DepartmentID: category.DepartmentID, CreatedAt: now},
				"StoreID", "IsBack", "FixtureType", "Name", "DepartmentID", "CreatedAt")
			if err != nil {
				return err
			}
			if created {
				locations.FosLocationsCreated++
			}

			if err := upsertStoreProduct(tx, storeID, p.ID, moq); err != nil {
				return err
			}
			added, err := insertProductLocation(tx, fosID, p, now)
			if err != nil {
				return err
			}
			locations.FosAdded += added
		}
	}

	report.Locations = locations
	return nil
}

// findOrCreateLocation finds a location by (store_id, is_back, name), or inserts it with only
// the given columns (id is GENERATED ALWAYS, so it is never sent). Returns id and "created".
func findOrCreateLocation(tx *gorm.DB, loc model.StoreLocation, columns ...string) (uint, bool, error) {
	var existing model.StoreLocation
	res := tx.Where("store_id = ? AND is_back = ? AND name = ?", loc.StoreID, loc.IsBack, *loc.Name).Limit(1).Find(&existing)
	if res.Error != nil {
		return 0, false, res.Error
	}
	if res.RowsAffected > 0 {
		return existing.ID, false, nil
	}
	if err := tx.Select(columns).Create(&loc).Error; err != nil {
		return 0, false, err
	}
	return loc.ID, true, nil
}

// upsertStoreProduct inserts (store_id, product_id, moq); if the row exists only moq is updated.
func upsertStoreProduct(tx *gorm.DB, storeID, productID uint, moq float64) error {
	return tx.Exec(`
		INSERT INTO store_products (store_id, product_id, moq) VALUES (?, ?, ?)
		ON CONFLICT (store_id, product_id) DO UPDATE SET moq = EXCLUDED.moq`, storeID, productID, moq).Error
}

// insertProductLocation adds the product to a location; an existing row is left untouched.
// Returns 1 when a row was inserted, 0 when it already existed.
func insertProductLocation(tx *gorm.DB, locationID uint, p model.Product, now time.Time) (int64, error) {
	res := tx.Exec(`
		INSERT INTO product_location (product_id, location_id, sku, vendor_key, retail_price, uom_buy, uom_sell, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT DO NOTHING`,
		p.ID, locationID, p.Sku, p.VendorKey, p.RetailPrice, p.UomBuy, p.UomSell, now)
	return res.RowsAffected, res.Error
}

// --- small helpers -----------------------------------------------------------

// dbValue makes a value read from the DB easy to compare and print:
// real (float32) => float64 with the same short digits (45.99, not 45.9900016),
// int32 => int64, []byte => string. NULL stays nil.
func dbValue(v any) any {
	switch t := v.(type) {
	case float32:
		f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(t), 'f', -1, 32), 64)
		return f
	case int32:
		return int64(t)
	case []byte:
		return string(t)
	}
	return v
}

// valueChanged copies the Laravel check: two numbers => differ by more than 0.00001;
// anything else => compare as text (NULL = "").
func valueChanged(oldValue, newValue any) bool {
	a, aNum := asNumber(oldValue)
	b, bNum := asNumber(newValue)
	if aNum && bNum {
		return math.Abs(a-b) > 0.00001
	}
	return valueText(oldValue) != valueText(newValue)
}

func asNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}

func valueText(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int32:
		return int64(t)
	case int64:
		return t
	}
	return 0
}
