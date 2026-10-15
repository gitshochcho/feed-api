package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterProductUpcRoutes registers all /product-upcs endpoints.
func RegisterProductUpcRoutes(rg *gin.RouterGroup, h *handler.ProductUpcHandler) {
	rg.POST("", h.Store)
}
