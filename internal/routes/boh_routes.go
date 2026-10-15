package routes

import (
	"github.com/gin-gonic/gin"

	"feed-api/internal/handler"
)

// RegisterBohRoutes registers all /boh endpoints.
func RegisterBohRoutes(rg *gin.RouterGroup, h *handler.BohHandler) {
	rg.POST("", h.Store)
}
