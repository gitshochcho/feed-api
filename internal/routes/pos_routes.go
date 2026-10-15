package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterPosRoutes registers all /pos endpoints.
func RegisterPosRoutes(rg *gin.RouterGroup, h *handler.PosHandler) {
	rg.POST("", h.Store)
}
