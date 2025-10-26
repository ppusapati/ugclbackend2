package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	dsn := os.Getenv("DB_DSN")
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Configure connection pool for optimal performance
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to get database instance:", err)
	}

	// Set maximum number of open connections to the database
	// Default is unlimited, but we set a reasonable limit based on expected load
	maxOpenConns := getEnvAsInt("DB_MAX_OPEN_CONNS", 100)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	// Set maximum number of idle connections in the pool
	// This keeps connections ready for reuse, reducing connection overhead
	maxIdleConns := getEnvAsInt("DB_MAX_IDLE_CONNS", 10)
	sqlDB.SetMaxIdleConns(maxIdleConns)

	// Set maximum lifetime of a connection
	// Ensures connections are periodically recreated, preventing stale connections
	connMaxLifetime := getEnvAsDuration("DB_CONN_MAX_LIFETIME", time.Hour)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// Set maximum idle time for a connection
	// Closes idle connections after this duration to free resources
	connMaxIdleTime := getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	log.Printf("Database connection pool configured: MaxOpen=%d, MaxIdle=%d, MaxLifetime=%v, MaxIdleTime=%v",
		maxOpenConns, maxIdleConns, connMaxLifetime, connMaxIdleTime)

	// Run migrations
	if err := Migrations(DB); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Seed permissions and roles
	// SeedPermissions()

	// Seed business verticals
	// SeedBusinessVerticals()

	// Seed sites
	SeedSites()
}

// getEnvAsInt reads an environment variable as int with a default value
func getEnvAsInt(key string, defaultVal int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultVal
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid value for %s, using default %d", key, defaultVal)
		return defaultVal
	}
	return value
}

// getEnvAsDuration reads an environment variable as duration with a default value
// Expected format: "1h", "30m", "90s", etc.
func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultVal
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid duration for %s, using default %v", key, defaultVal)
		return defaultVal
	}
	return value
}
