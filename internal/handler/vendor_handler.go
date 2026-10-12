package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/repository"
)

type VendorHandler struct {
	repo *repository.VendorRepository
}

func NewVendorHandler(repo *repository.VendorRepository) *VendorHandler {
	return &VendorHandler{repo: repo}
}

func (h *VendorHandler) CreateVendor(c *gin.Context) {
	var req dto.CreateVendorRequest

	// 1. Validate JSON input
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. 'name' and 'external_id' are required."})
		return
	}

	// 2. Upsert the vendor by external_id and link it to every store (one transaction)
	result, err := h.repo.Upsert(c.Request.Context(), req.Name, req.ExternalID)
	if err != nil {
		// external_id is the upsert key, so a duplicate here means the name belongs to another vendor.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "A vendor with this name already exists."})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save vendor"})
		return
	}

	// 3. Return success: 201 when created, 200 when updated or unchanged
	status, message := http.StatusOK, "Vendor unchanged"
	switch result.Action {
	case "inserted":
		status, message = http.StatusCreated, "Vendor created successfully"
	case "updated":
		message = "Vendor updated successfully"
	}
	c.JSON(status, gin.H{"message": message, "data": result})
}
