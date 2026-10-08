package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"feed-api/internal/config"
	"feed-api/internal/model"
)

func ConnectDB(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Converts driver errors (e.g. unique violations) into gorm.ErrDuplicatedKey etc.
		TranslateError: true,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\n", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v\n", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}

	log.Println("Successfully connected to PostgreSQL!")
	return db
}

// Migrate creates/updates tables for all registered models.
func Migrate(db *gorm.DB) {
	if err := db.AutoMigrate(&model.Vendor{}); err != nil {
		log.Fatalf("Database migration failed: %v\n", err)
	}
	log.Println("Database migration completed")
}
