package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterVendorRoutes registers all /vendors endpoints.
func RegisterVendorRoutes(rg *gin.RouterGroup, h *handler.VendorHandler) {
	rg.POST("", h.CreateVendor)
}
