package dto

// =============================================================================
// INPUT
// =============================================================================

// ProductRequest is one product feed row (one line of the old products file).
// FlexString does "STEP 1 — CLEAN INPUT" while decoding (trim, numbers -> text,
// missing / null / array / object -> ""). The binding tags are "STEP 2 — VALIDATE".
type ProductRequest struct {
	Level1Key      FlexString `json:"level1Key" binding:"max=255"`
	Level2Key      FlexString `json:"level2Key" binding:"max=255"`
	Level3Key      FlexString `json:"level3Key" binding:"max=255"`
	Level4Key      FlexString `json:"level4Key" binding:"max=255"`
	Level5Key      FlexString `json:"level5Key" binding:"max=255"`
	Level6Key      FlexString `json:"level6Key"` // ignored
	Sku            FlexString `json:"sku" binding:"required,max=255"`
	Name           FlexString `json:"name"`
	Brand          FlexString `json:"brand"`
	Status         FlexString `json:"status" binding:"omitempty,oneof=0 1"`
	VendorKey      FlexString `json:"vendorKey"`
	UomBuy         FlexString `json:"uomBuy"`
	BuyQty         FlexString `json:"buyQty" binding:"omitempty,numeric"`
	UomSell        FlexString `json:"uomSell"`
	InnerPack      FlexString `json:"innerPack" binding:"omitempty,numeric"`
	MasterPack     FlexString `json:"masterPack" binding:"omitempty,numeric"`
	Image          FlexString `json:"image"`
	PrimaryUpc     FlexString `json:"primaryUpc"`
	RetailPrice    FlexString `json:"retailPrice" binding:"omitempty,numeric"`
	UnitLandedCost FlexString `json:"unitLandedCost" binding:"omitempty,numeric"`
	SkuType        FlexString `json:"skuType"`
	ParentSku      FlexString `json:"parentSku"`
	QtyPerParent   FlexString `json:"qtyPerParent" binding:"omitempty,numeric"`
	ReferenceSku   FlexString `json:"referenceSku"`
	ShelfDays      FlexString `json:"shelfDays" binding:"omitempty,numeric"`
	Moq            FlexString `json:"moq" binding:"omitempty,numeric"` // store_products.moq, empty => 1
}

// ProductRow is the clean row after STEP 1 + 2 (plain strings).
type ProductRow struct {
	Levels         [5]string // level1Key..level5Key
	Sku            string
	Name           string
	Brand          string
	Status         string
	VendorKey      string
	UomBuy         string
	BuyQty         string
	UomSell        string
	InnerPack      string
	MasterPack     string
	Image          string
	PrimaryUpc     string
	RetailPrice    string
	UnitLandedCost string
	SkuType        string
	ParentSku      string
	QtyPerParent   string
	ReferenceSku   string
	ShelfDays      string
	Moq            string
}

// ToRow converts the decoded request into a plain-string row (level6Key is dropped).
func (r ProductRequest) ToRow() ProductRow {
	return ProductRow{
		Levels:         [5]string{string(r.Level1Key), string(r.Level2Key), string(r.Level3Key), string(r.Level4Key), string(r.Level5Key)},
		Sku:            string(r.Sku),
		Name:           string(r.Name),
		Brand:          string(r.Brand),
		Status:         string(r.Status),
		VendorKey:      string(r.VendorKey),
		UomBuy:         string(r.UomBuy),
		BuyQty:         string(r.BuyQty),
		UomSell:        string(r.UomSell),
		InnerPack:      string(r.InnerPack),
		MasterPack:     string(r.MasterPack),
		Image:          string(r.Image),
		PrimaryUpc:     string(r.PrimaryUpc),
		RetailPrice:    string(r.RetailPrice),
		UnitLandedCost: string(r.UnitLandedCost),
		SkuType:        string(r.SkuType),
		ParentSku:      string(r.ParentSku),
		QtyPerParent:   string(r.QtyPerParent),
		ReferenceSku:   string(r.ReferenceSku),
		ShelfDays:      string(r.ShelfDays),
		Moq:            string(r.Moq),
	}
}

// =============================================================================
// OUTPUT
// =============================================================================

// ProductImportReport is what the DB transaction did (steps 6..8).
// Brand is null when no brand was written. InactiveCleanup is set only for
// status I, Locations only for status A.
type ProductImportReport struct {
	Brand           *BrandReport           `json:"brand"`
	Product         ProductReport          `json:"product"`
	InactiveCleanup *InactiveCleanupReport `json:"inactiveCleanup"`
	Locations       *LocationsReport       `json:"locations"`
}

// ProductImportResult is the full success response.
// The embedded report's fields appear at the top level, like PHP's `[...] + $result`.
type ProductImportResult struct {
	Status        string    `json:"status"`
	Shifted       bool      `json:"shifted"`
	LevelsUsed    [5]string `json:"levelsUsed"`
	CategoryID    uint      `json:"categoryId"`
	ProductStatus string    `json:"productStatus"`
	ProductImportReport
}

// BrandReport is the result of STEP 6. Action is "inserted", "updated" or "unchanged".
// Before holds the old name, only when updated.
type BrandReport struct {
	ID         uint    `json:"id"`
	ExternalID string  `json:"external_id"`
	Action     string  `json:"action"`
	Before     *string `json:"before"`
}

// ProductReport is the result of STEP 7. Changes lists only the mapped columns that changed.
type ProductReport struct {
	ID      uint                   `json:"id"`
	Sku     string                 `json:"sku"`
	Action  string                 `json:"action"`
	Changes map[string]FieldChange `json:"changes"`
}

// InactiveCleanupReport is the result of STEP 8a.
type InactiveCleanupReport struct {
	FosRemovalEnabled          bool  `json:"fosRemovalEnabled"`
	FosProductLocationsRemoved int64 `json:"fosProductLocationsRemoved"`
	EmptyFosLocationsRemoved   int64 `json:"emptyFosLocationsRemoved"`
	PicklistProductsRemoved    int64 `json:"picklistProductsRemoved"`
}

// LocationsReport is the result of STEP 8b.
type LocationsReport struct {
	StoresChecked         int   `json:"storesChecked"`
	StoresAlreadyComplete int   `json:"storesAlreadyComplete"`
	BackRoomsCreated      int   `json:"backRoomsCreated"`
	BosAdded              int64 `json:"bosAdded"`
	FosLocationsCreated   int   `json:"fosLocationsCreated"`
	FosAdded              int64 `json:"fosAdded"`
}
