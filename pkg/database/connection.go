package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	// DB is the global database connection
	DB *gorm.DB
)

// InitDatabase initializes the database connection with the given configuration
func InitDatabase(config DatabaseConfig) error {
	var dialector gorm.Dialector

	switch config.Driver {
	case "mysql":
		dialector = mysql.Open(config.DSN)
	case "postgres", "postgresql":
		dialector = postgres.Open(config.DSN)
	case "sqlite":
		dialector = sqlite.Open(config.DSN)
	default:
		return fmt.Errorf("unsupported database driver: %s", config.Driver)
	}

	// Configure GORM logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	var err error
	DB, err = gorm.Open(dialector, gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to %s database successfully", config.Driver)

	return nil
}


// CloseDatabase closes the database connection
func CloseDatabase() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// GetDatabase returns the current database connection
func GetDatabase() *gorm.DB {
	return DB
}

// HealthCheck performs a database health check
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database connection is nil")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns database connection statistics
func GetConnectionStats() map[string]interface{} {
	if DB == nil {
		return map[string]interface{}{"error": "database connection is nil"}
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	stats := sqlDB.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}
}

// AreaCount represents echoarea message count
type AreaCount struct {
	EchoareaID int64 `json:"echoarea_id"`
	Count      int64 `json:"count"`
}

// GetAllEchoareaCounts returns message counts for all echoareas in a single query
func GetAllEchoareaCounts() (map[int64]int64, error) {
	if DB == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	var counts []AreaCount
	err := DB.Model(&Echomail{}).
		Select("echoarea_id, COUNT(*) as count").
		Group("echoarea_id").
		Find(&counts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get echoarea counts: %w", err)
	}

	result := make(map[int64]int64)
	for _, count := range counts {
		result[count.EchoareaID] = count.Count
	}

	return result, nil
}

// GetNetmailCount returns total netmail count
func GetNetmailCount() (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database connection is nil")
	}

	var count int64
	err := DB.Model(&Netmail{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get netmail count: %w", err)
	}

	return count, nil
}
