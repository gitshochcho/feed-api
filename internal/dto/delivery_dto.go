package dto

import "time"

// =============================================================================
// INPUT
// =============================================================================

// DeliveryRequest is one deliveries feed row (old file line:
// Timestamp|ASN|ETA|DU|DU_Number|SKU|Description|Expected_Cases|Expected_Units|Damaged|From|Store_ID).
// FlexString does "STEP 1 — CLEAN INPUT" (trim; missing / null / array / object -> "").
// The binding tags are part of "STEP 2 — VALIDATE" (asn range and eta are checked in the handler).
// Fields are named like their JSON keys, because validation errors use the field name ("storeId").
type DeliveryRequest struct {
	Timestamp          FlexString `json:"timestamp"` // accepted, ignored
	Asn                FlexString `json:"asn" binding:"required,number"`
	Eta                FlexString `json:"eta" binding:"required"`
	DeliveryUnitCode   FlexString `json:"deliveryUnitCode" binding:"required,max=255"`
	DeliveryUnitNumber FlexString `json:"deliveryUnitNumber"` // accepted, ignored
	Sku                FlexString `json:"sku" binding:"required,max=255"`
	Description        FlexString `json:"description" binding:"max=255"`
	ExpectedCases      FlexString `json:"expectedCases" binding:"omitempty,numeric"`
	ExpectedUnits      FlexString `json:"expectedUnits" binding:"omitempty,numeric"`
	DamagedCases       FlexString `json:"damagedCases" binding:"omitempty,numeric"`
	From               FlexString `json:"from" binding:"max=255"`
	StoreId            FlexString `json:"storeId" binding:"required,number"`
}

// DeliveryRow is the converted row after STEP 3 (what the repository saves).
type DeliveryRow struct {
	Asn              int64
	Eta              time.Time // UTC
	From             string
	DeliveryUnitCode string
	Description      string
	ExpectedCases    int
	ExpectedUnits    int
	DamagedCases     int
}

// =============================================================================
// OUTPUT
// =============================================================================

// DeliveryReport is what the DB transaction did (STEP 6).
// When the delivery has already started, Blocked is true and nothing was written
// (the Blocked* fields are never sent to the client).
type DeliveryReport struct {
	Delivery        DeliveryRowReport        `json:"delivery"`
	DeliveryUnit    DeliveryUnitReport       `json:"deliveryUnit"`
	DeliveryProduct DeliveryProductRowReport `json:"deliveryProduct"`

	Blocked           bool   `json:"-"`
	BlockedDeliveryID uint   `json:"-"`
	BlockedStatus     string `json:"-"`
}

// DeliveryRowReport is the result of STEP 6a. Action is "inserted", "updated" or "unchanged".
type DeliveryRowReport struct {
	ID      uint                   `json:"id"`
	Asn     int64                  `json:"asn"`
	Action  string                 `json:"action"`
	Changes map[string]FieldChange `json:"changes"`
}

// DeliveryUnitReport is the result of STEP 6b. Action is "inserted" or "unchanged".
type DeliveryUnitReport struct {
	ID     uint   `json:"id"`
	Code   string `json:"code"`
	Action string `json:"action"`
}

// DeliveryProductRowReport is the result of STEP 6c. Action is "inserted", "updated" or "unchanged".
type DeliveryProductRowReport struct {
	ID      uint                   `json:"id"`
	Action  string                 `json:"action"`
	Changes map[string]FieldChange `json:"changes"`
}

// DeliveryResult is the full success response.
// The embedded report's fields appear at the top level, like PHP's `[...] + $result`.
type DeliveryResult struct {
	Status  string             `json:"status"`
	Store   DeliveryStoreRef   `json:"store"`
	Product DeliveryProductRef `json:"product"`
	DeliveryReport
	PostProcessing any `json:"postProcessing"`
}

type DeliveryStoreRef struct {
	ExternalID int64 `json:"externalId"`
	ID         uint  `json:"id"`
}

type DeliveryProductRef struct {
	ID  uint   `json:"id"`
	Sku string `json:"sku"`
}
