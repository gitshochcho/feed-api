package model

import "time"

type Vendor struct {
	Name       string    `json:"name" binding:"required"`
	ExternalID string    `json:"external_id" binding:"required"`
	CreatedAt  time.Time `json:"created_at"`
}