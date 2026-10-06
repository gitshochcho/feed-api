package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"feed-api/internal/config"
)

func ConnectDB(cfg *config.Config) *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\n", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}

	log.Println("Successfully connected to PostgreSQL!")
	return db
}