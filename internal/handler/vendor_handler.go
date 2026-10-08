package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
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

	vendor := model.Vendor{
		Name:       req.Name,
		ExternalID: req.ExternalID,
	}

	// 2. Insert into database (GORM sets ID and CreatedAt)
	if err := h.repo.Create(c.Request.Context(), &vendor); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "A vendor with this name or external_id already exists."})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert vendor into database"})
		return
	}

	// 3. Return success
	c.JSON(http.StatusCreated, gin.H{
		"message": "Vendor created successfully",
		"data":    vendor,
	})
}
