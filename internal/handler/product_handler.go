package handler

import (
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/model"
	"feed-api/internal/repository"
)

type ProductHandler struct {
	repo *repository.ProductRepository
}

func NewProductHandler(repo *repository.ProductRepository) *ProductHandler {
	return &ProductHandler{repo: repo}
}

// Store imports ONE product feed row (spec 1.4, steps 1..8).
func (h *ProductHandler) Store(c *gin.Context) {
	var req dto.ProductRequest

	// 1. CLEAN INPUT + 2. VALIDATE
	// dto.FlexString trims every value and turns numbers into text while decoding;
	// missing / null / array / object become "". Then the binding tags check:
	// sku required + max 255, status empty or 0/1, the 7 number fields + moq numeric, level keys max 255.
	// Any failure => 422, nothing is saved.
	if err := c.ShouldBindJSON(&req); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			errs := validationMessages(verrs)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": firstMessage(verrs, errs), "errors": errs})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body"})
		return
	}
	row := req.ToRow()

	// 3a. STATUS
	// CURRENT RULE (ENABLED): "1" => A (active); anything else, including "0" and empty => I.
	status := "I"
	if row.Status == "1" {
		status = "A"
	}
	// OLD FILE RULE (DISABLED): the old file mapped "0" => A, anything else => I.
	// Keep it commented while the external team sends 1 = active. If they keep the
	// old meaning (0 = active), replace the CURRENT RULE above with:
	//
	//	status := "I"
	//	if row.Status == "0" {
	//		status = "A"
	//	}

	// 3b. NUMBERS: empty => 0. Whole numbers drop the decimal part ("12.7" => 12).
	// 3c. innerPack: 0 or empty => 1.
	innerPack := toInt(row.InnerPack)
	if innerPack == 0 {
		innerPack = 1
	}

	// 3d. MOQ (store_products.moq, written in STEP 8b): missing or empty => 1, otherwise the number sent.
	moq := 1.0
	if row.Moq != "" {
		moq = toFloat(row.Moq)
	}

	// 4. SHIFT RULE
	// Only level3Key is checked. If it is a shift key: L1 stays, L2 = L3, L3 = L4, L4 = L5, L5 = "".
	// The original L2 is dropped (same rule as the hierarchy feed, so the keys match the category).
	levels := row.Levels
	shifted := false
	if isShiftKey(levels[2]) {
		levels = [5]string{levels[0], levels[2], levels[3], levels[4], ""}
		shifted = true
	}

	// 5. FIND THE CATEGORY (all 5 keys must match exactly)
	// Not found => 422, nothing saved: the hierarchy feed must run first.
	category, err := h.repo.FindCategory(c.Request.Context(), levels)
	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import product"})
		return
	}
	if category == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":      "Category not found for these level keys. Send the product hierarchy first.",
			"levelsUsed": levels,
			"shifted":    shifted,
		})
		return
	}

	// 6..8. SAVE IN ONE TRANSACTION (brand, product, inactive cleanup or BOS/FOS locations)
	// brand_id and the fixed values are set by the repository.
	product := model.Product{
		Sku:            row.Sku,
		CategoryID:     category.ID,
		Name:           row.Name,
		Brand:          row.Brand,
		Status:         status,
		VendorKey:      row.VendorKey,
		UomBuy:         row.UomBuy,
		BuyQty:         toFloat(row.BuyQty),
		UomSell:        row.UomSell,
		InnerPack:      innerPack,
		MasterPack:     toInt(row.MasterPack),
		Image:          row.Image,
		PrimaryUpc:     row.PrimaryUpc,
		RetailPrice:    toFloat(row.RetailPrice),
		UnitLandedCost: toFloat(row.UnitLandedCost),
		SkuType:        row.SkuType,
		ParentSku:      row.ParentSku,
		QtyPerParent:   toFloat(row.QtyPerParent),
		ReferenceSku:   row.ReferenceSku,
		ShelfDays:      toInt(row.ShelfDays),
	}

	report, err := h.repo.Import(c.Request.Context(), product, category, moq)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "Conflicting row (e.g. a brand with this name under another key, or the same sku saved at the same time). Nothing was saved."})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import product"})
		return
	}

	// 9. Return what happened (spec 1.5)
	c.JSON(http.StatusOK, dto.ProductImportResult{
		Status:              "processed",
		Shifted:             shifted,
		LevelsUsed:          levels,
		CategoryID:          category.ID,
		ProductStatus:       status,
		ProductImportReport: *report,
	})
}

// toFloat: decimals, empty => 0 (the value was already validated as numeric).
func toFloat(v string) float64 {
	if v == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

// toInt: whole numbers, empty => 0, decimal part dropped like PHP's (int) cast ("12.7" => 12).
func toInt(v string) int {
	return int(math.Trunc(toFloat(v)))
}
