package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	dsn := os.Getenv("DB_DSN")
	gormSlowQueryThreshold := getEnvAsDuration("DB_SLOW_QUERY_THRESHOLD", 200*time.Millisecond)
	gormLogLevel := getEnvAsGormLogLevel("DB_GORM_LOG_LEVEL", "warn")

	gormConfig := &gorm.Config{
		Logger: gormlogger.New(
			log.New(os.Stdout, "", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             gormSlowQueryThreshold,
				LogLevel:                  gormLogLevel,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
		// Prepared statements reduce parse/plan overhead for repeated API queries.
		PrepareStmt: getEnvAsBool("DB_PREPARE_STMT", true),
		// Per-write implicit transactions add overhead; keep toggleable for safe rollout.
		SkipDefaultTransaction: getEnvAsBool("DB_SKIP_DEFAULT_TX", true),
	}

	DB, err = gorm.Open(postgres.Open(dsn), gormConfig)
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

	// Set maximum number of idle connections in the pool.
	// Raised from 10 → 25 so concurrent auth-middleware queries (which previously
	// each needed 3 DB round-trips) no longer wait for a free connection.
	maxIdleConns := getEnvAsInt("DB_MAX_IDLE_CONNS", 25)
	sqlDB.SetMaxIdleConns(maxIdleConns)

	// Set maximum lifetime of a connection
	// Ensures connections are periodically recreated, preventing stale connections
	connMaxLifetime := getEnvAsDuration("DB_CONN_MAX_LIFETIME", time.Hour)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// Set maximum idle time for a connection
	// Closes idle connections after this duration to free resources
	connMaxIdleTime := getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	// Periodic health-check ping: proactively recycles stale connections so they
	// are not handed out to handlers and cause first-query latency spikes.
	// DB_HEALTH_CHECK_PERIOD=0 disables it (useful in environments with managed pools).
	healthCheckPeriod := getEnvAsDuration("DB_HEALTH_CHECK_PERIOD", 30*time.Second)
	if healthCheckPeriod > 0 {
		go func() {
			ticker := time.NewTicker(healthCheckPeriod)
			defer ticker.Stop()
			for range ticker.C {
				if pingErr := sqlDB.Ping(); pingErr != nil {
					log.Printf("[DB HEALTH] ping failed: %v", pingErr)
				}
			}
		}()
	}

	log.Printf("Database connection pool configured: MaxOpen=%d, MaxIdle=%d, MaxLifetime=%v, MaxIdleTime=%v, HealthCheckPeriod=%v",
		maxOpenConns, maxIdleConns, connMaxLifetime, connMaxIdleTime, healthCheckPeriod)
	log.Printf("GORM performance settings: PrepareStmt=%t, SkipDefaultTransaction=%t",
		gormConfig.PrepareStmt, gormConfig.SkipDefaultTransaction)
	log.Printf("GORM SQL logging: level=%v, slow_threshold=%v", gormLogLevel, gormSlowQueryThreshold)

	// Run migrations
	if err := Migrations(DB); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

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

// getEnvAsBool reads common truthy/falsey environment values with a default fallback.
func getEnvAsBool(key string, defaultVal bool) bool {
	valueStr := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if valueStr == "" {
		return defaultVal
	}

	switch valueStr {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		log.Printf("Warning: Invalid boolean for %s, using default %t", key, defaultVal)
		return defaultVal
	}
}

func getEnvAsGormLogLevel(key, defaultVal string) gormlogger.LogLevel {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		value = strings.TrimSpace(strings.ToLower(defaultVal))
	}

	switch value {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	case "warn", "warning":
		return gormlogger.Warn
	default:
		log.Printf("Warning: Invalid gorm log level for %s, using %s", key, defaultVal)
		return gormlogger.Warn
	}
}
