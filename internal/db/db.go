package db

import (
	"log"
	"prerender-shield/internal/config"
	"prerender-shield/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB initializes the database connection
func InitDB(cfg *config.Config) error {
	if cfg.Storage.Type != "postgres" {
		log.Println("Database type is not postgres, skipping database initialization")
		return nil
	}

	dsn := cfg.Storage.PostgresURL
	if dsn == "" {
		log.Println("PostgresURL is empty, skipping database initialization")
		return nil
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	log.Println("Successfully connected to database")

	// Auto Migrate
	return AutoMigrate()
}

// AutoMigrate runs database migrations
func AutoMigrate() error {
	if DB == nil {
		return nil
	}
	return DB.AutoMigrate(
		&models.Site{},
		&models.WafConfig{},
		&models.BlockedCountry{},
		&models.IPWhitelist{},
		&models.IPBlacklist{},
		&models.AccessLog{},
	)
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
