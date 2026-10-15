package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterDeliveryRoutes registers all /deliveries endpoints.
func RegisterDeliveryRoutes(rg *gin.RouterGroup, h *handler.DeliveryHandler) {
	rg.POST("", h.Store)
}
