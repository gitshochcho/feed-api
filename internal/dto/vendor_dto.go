package dto

type CreateVendorRequest struct {
	Name       string `json:"name" binding:"required"`
	ExternalID string `json:"external_id" binding:"required"`
}
