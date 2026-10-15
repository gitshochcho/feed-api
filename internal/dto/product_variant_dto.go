package dto

// =============================================================================
// INPUT
// =============================================================================

// ProductVariantRequest is one product_variants feed row (old file line: Parent_Sku|Variant_SKU).
// FlexString does "STEP 1 — CLEAN INPUT" (trim; missing / null / array / object -> "").
// Send both as JSON strings: a number loses its leading zeros (05100007508 -> "5100007508").
// The binding tags are "STEP 2 — VALIDATE" ("  " is trimmed to "" and fails required).
type ProductVariantRequest struct {
	ParentSku  FlexString `json:"parentSku" binding:"required,max=255"`
	VariantSku FlexString `json:"variantSku" binding:"required,max=255"`
}

// =============================================================================
// OUTPUT
// =============================================================================

// VariantRowReport is the result of STEP 4. Action is "inserted" or "unchanged";
// VariantStatus is the row's status ("related" on insert, kept as it is otherwise).
type VariantRowReport struct {
	ID            uint   `json:"id"`
	Action        string `json:"action"`
	VariantStatus string `json:"variantStatus"`
}

// LinkedProduct is another product that already uses the same variant code (information only).
type LinkedProduct struct {
	ProductID uint   `json:"productId"`
	Sku       string `json:"sku"`
}

// ProductVariantResult is the full success response. AlsoLinkedTo is [] when the code is not shared.
type ProductVariantResult struct {
	Status       string                   `json:"status"`
	Product      ProductVariantProductRef `json:"product"`
	Variant      string                   `json:"variant"`
	VariantRow   VariantRowReport         `json:"variantRow"`
	AlsoLinkedTo []LinkedProduct          `json:"alsoLinkedTo"`
}

type ProductVariantProductRef struct {
	ID  uint   `json:"id"`
	Sku string `json:"sku"`
}
