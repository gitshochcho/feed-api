package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterProductRoutes registers all /products endpoints.
func RegisterProductRoutes(rg *gin.RouterGroup, h *handler.ProductHandler) {
	rg.POST("", h.Store)
}
