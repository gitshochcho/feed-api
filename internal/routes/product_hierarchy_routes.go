package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterProductHierarchyRoutes registers all /product-hierarchies endpoints.
func RegisterProductHierarchyRoutes(rg *gin.RouterGroup, h *handler.ProductHierarchyHandler) {
	rg.POST("", h.Store)
}
