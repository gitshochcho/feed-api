package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"feed-api/internal/handler"
	"feed-api/internal/repository"
)

// SetupRouter wires repositories and handlers, then registers every route group.
func SetupRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Vendors
	vendorRepo := repository.NewVendorRepository(db)
	vendorHandler := handler.NewVendorHandler(vendorRepo)
	RegisterVendorRoutes(r.Group("/vendors"), vendorHandler)

	return r
}
