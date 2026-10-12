package dto

import "feed-api/internal/model"

type CreateVendorRequest struct {
	Name       string `json:"name" binding:"required"`
	ExternalID string `json:"external_id" binding:"required"`
}

// VendorUpsertResult is what POST /vendors did.
// Action is "inserted", "updated" or "unchanged". Before is set only when updated.
type VendorUpsertResult struct {
	Vendor          model.Vendor  `json:"vendor"`
	Action          string        `json:"action"`
	Before          *VendorBefore `json:"before"`
	StoreLinksAdded int64         `json:"storeLinksAdded"`
}

type VendorBefore struct {
	Name string `json:"name"`
}
