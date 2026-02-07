package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func InitDB() error {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "trader")
	dbPassword := getEnv("DB_PASSWORD", "traderpass")
	dbName := getEnv("DB_NAME", "algo_trader")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)

	if err = migrateDMA20Column(); err != nil {
		log.Printf("Migration warning (dma_20): %v", err)
	}

	log.Println("Database connection established")
	return nil
}

// migrateDMA20Column adds dma_20 column to stock_prices if it doesn't exist (for existing DBs).
func migrateDMA20Column() error {
	var count int
	err := DB.QueryRow(`
		SELECT COUNT(*) FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stock_prices' AND COLUMN_NAME = 'dma_20'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = DB.Exec("ALTER TABLE stock_prices ADD COLUMN dma_20 DECIMAL(15,2) NULL COMMENT '20-day simple moving average of close' AFTER volume")
	return err
}

func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
