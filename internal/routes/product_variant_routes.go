package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterProductVariantRoutes registers all /product-variants endpoints.
func RegisterProductVariantRoutes(rg *gin.RouterGroup, h *handler.ProductVariantHandler) {
	rg.POST("", h.Store)
}
