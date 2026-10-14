package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"feed-api/internal/dto"
	"feed-api/internal/repository"
)

type ProductVariantHandler struct {
	repo *repository.ProductVariantRepository
}

func NewProductVariantHandler(repo *repository.ProductVariantRepository) *ProductVariantHandler {
	return &ProductVariantHandler{repo: repo}
}

// Store imports ONE product_variants feed row (old file line: Parent_Sku|Variant_SKU).
// Only product_variant is written, and only by INSERT (never update, never delete).
func (h *ProductVariantHandler) Store(c *gin.Context) {
	var req dto.ProductVariantRequest
	ctx := c.Request.Context()

	// 1. CLEAN INPUT + 2. VALIDATE
	// dto.FlexString trims both values while decoding; missing / null / array / object become "".
	// Then the binding tags check: both required, max 255. Any failure => 422, nothing is saved
	// (the CLI silently skipped empty rows).
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
	parentSku, variant := string(req.ParentSku), string(req.VariantSku)

	// 3. FIND THE PARENT PRODUCT by exact sku. It must be ACTIVE (status A).
	// Not found => 422. Not active => 422 (the CLI silently skipped both).
	product, err := h.repo.FindProductBySku(ctx, parentSku)
	if err != nil {
		h.serverError(c, err)
		return
	}
	if product == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Parent product not found for this SKU.", "parentSku": parentSku})
		return
	}
	if product.Status != "A" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":     fmt.Sprintf("Parent product is not active (status %s).", product.Status),
			"parentSku": parentSku,
		})
		return
	}

	// 4. LINK (one transaction): insert (product_id, variant) if it does not exist, else keep it.
	variantRow, err := h.repo.Link(ctx, product.ID, variant)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// Two requests inserted the same new link at the same moment. A retry gives "unchanged".
			c.JSON(http.StatusConflict, gin.H{"error": "This link was saved by another request at the same time. Please retry."})
			return
		}
		h.serverError(c, err)
		return
	}

	// 5. Information only: other products that already use this variant code.
	alsoLinkedTo, err := h.repo.OtherProductsWithVariant(ctx, product.ID, variant)
	if err != nil {
		h.serverError(c, err)
		return
	}

	// 6. Return what happened (spec 1.5)
	c.JSON(http.StatusOK, dto.ProductVariantResult{
		Status:       "processed",
		Product:      dto.ProductVariantProductRef{ID: product.ID, Sku: product.Sku},
		Variant:      variant,
		VariantRow:   *variantRow,
		AlsoLinkedTo: alsoLinkedTo,
	})
}

// serverError logs the real error and sends a generic 500 (never the DB error text).
func (h *ProductVariantHandler) serverError(c *gin.Context, err error) {
	log.Printf("Database error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import product variant"})
}
