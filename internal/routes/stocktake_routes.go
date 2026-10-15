package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterStockTakeRoutes registers all /stocktake endpoints.
func RegisterStockTakeRoutes(rg *gin.RouterGroup, h *handler.StockTakeHandler) {
	rg.POST("", h.Store)
}
