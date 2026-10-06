package handler

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"feed-api/internal/model"
)

type VendorHandler struct {
	db *sql.DB
}

func NewVendorHandler(db *sql.DB) *VendorHandler {
	return &VendorHandler{db: db}
}

func (h *VendorHandler) CreateVendor(c *gin.Context) {
	var vendor model.Vendor

	// 1. Validate JSON input
	if err := c.ShouldBindJSON(&vendor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. 'name' and 'external_id' are required."})
		return
	}

	// 2. Set the current timestamp
	vendor.CreatedAt = time.Now()

	// 3. Insert into database (now including created_at)
	query := `INSERT INTO vendors (name, external_id, created_at) VALUES ($1, $2, $3) RETURNING id`
	var id int

	err := h.db.QueryRow(query, vendor.Name, vendor.ExternalID, vendor.CreatedAt).Scan(&id)
	if err != nil {
		log.Printf("Database error: %v", err)
		
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			c.JSON(http.StatusConflict, gin.H{"error": "A vendor with this external_id already exists."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert vendor into database", "details": err.Error()})
		return
	}

	// 4. Return success
	c.JSON(http.StatusCreated, gin.H{
		"message": "Vendor created successfully",
		"data": gin.H{
			"id":          id,
			"name":        vendor.Name,
			"external_id": vendor.ExternalID,
			"created_at":  vendor.CreatedAt,
		},
	})
}