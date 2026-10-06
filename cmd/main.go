package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"feed-api/internal/config"
	"feed-api/internal/database"
	"feed-api/internal/handler"
)

func main() {
	// 1. Load Config & Connect DB
	cfg := config.LoadConfig()
	db := database.ConnectDB(cfg)
	defer db.Close()

	// 2. Setup Gin Router
	r := gin.Default()

	// 3. Initialize Handler & Register Route
	vendorHandler := handler.NewVendorHandler(db)
	r.POST("/vendors", vendorHandler.CreateVendor)

	// 4. Start Server
	log.Printf("Server starting on port %s...", cfg.ServerPort)
	if err := r.Run(":"+cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}