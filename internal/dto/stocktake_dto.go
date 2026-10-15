package dto

// =============================================================================
// INPUT
// =============================================================================

// StockTakeRequest is one stocktake feed row (old file line: Store_ID|SKU|BOS_QTY|FOS_QTY).
// FlexString does "STEP 1 — CLEAN INPUT" (trim; missing / null / array / object -> "").
// The binding tags are "STEP 2 — VALIDATE" ("0" is a valid count).
// Fields are named like their JSON keys, because validation errors use the field name ("storeId").
type StockTakeRequest struct {
	StoreId FlexString `json:"storeId" binding:"required,number"`
	Sku     FlexString `json:"sku" binding:"required,max=255"`
	BosQty  FlexString `json:"bosQty" binding:"required,numeric"`
	FosQty  FlexString `json:"fosQty" binding:"required,numeric"`
}

// =============================================================================
// OUTPUT
// =============================================================================

// StockTakeCounts shows the received counts and the two conversions of STEP 3.
type StockTakeCounts struct {
	BosReceived     string `json:"bosReceived"`
	FosReceived     string `json:"fosReceived"`
	BosForLocations int    `json:"bosForLocations"`
	FosForLocations int    `json:"fosForLocations"`
	BosForStockTake int    `json:"bosForStockTake"`
	FosForStockTake int    `json:"fosForStockTake"`
}

// LocationQty is one product_location's case / units / qty (case and units can be NULL).
type LocationQty struct {
	Case  *int `json:"case"`
	Units *int `json:"units"`
	Qty   int  `json:"qty"`
}

// LocationQtyReport is one location changed (or not) in STEP 7a / 7b.
type LocationQtyReport struct {
	ID     uint        `json:"id"`
	Action string      `json:"action"`
	Before LocationQty `json:"before"`
	After  LocationQty `json:"after"`
}

// LocationsCountReport is the result of STEP 7a (FOS) or 7b (BOS). Locations is [] when there are none.
type LocationsCountReport struct {
	LocationCount int                 `json:"locationCount"`
	Cleared       bool                `json:"cleared"`
	Locations     []LocationQtyReport `json:"locations"`
}

// StockTakeRowReport is the result of STEP 7c. Before is set only when updated.
type StockTakeRowReport struct {
	ID     uint             `json:"id"`
	Action string           `json:"action"`
	Before *StockTakeBefore `json:"before"`
}

type StockTakeBefore struct {
	BosQty int `json:"bosQty"`
	FosQty int `json:"fosQty"`
}

// StockTakeReport is what the DB transaction did (STEP 7).
type StockTakeReport struct {
	Fos       LocationsCountReport `json:"fos"`
	Bos       LocationsCountReport `json:"bos"`
	StockTake StockTakeRowReport   `json:"stockTake"`
}

// StockTakeResult is the full success response.
// The embedded report's fields appear at the top level, like PHP's `[...] + $result`.
type StockTakeResult struct {
	Status  string              `json:"status"`
	Store   StockTakeStoreRef   `json:"store"`
	Product StockTakeProductRef `json:"product"`
	Counts  StockTakeCounts     `json:"counts"`
	StockTakeReport
}

type StockTakeStoreRef struct {
	ExternalID int64 `json:"externalId"`
	ID         uint  `json:"id"`
}

type StockTakeProductRef struct {
	ID  uint   `json:"id"`
	Sku string `json:"sku"`
}
