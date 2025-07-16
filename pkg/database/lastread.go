package database

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	
	// Use pure Go SQLite driver instead of CGO-based one
	_ "modernc.org/sqlite"
)

var (
	// LastReadDB is the separate SQLite database for lastread values
	LastReadDB *gorm.DB
)

// LastRead represents a user's last read position in an area
type LastRead struct {
	ID           int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username     string `gorm:"column:username;not null;index" json:"username"`
	AreaName     string `gorm:"column:area_name;not null;index" json:"area_name"`
	LastReadMsg  uint32 `gorm:"column:last_read_msg;not null;default:0" json:"last_read_msg"`
	HighReadMsg  uint32 `gorm:"column:high_read_msg;not null;default:0" json:"high_read_msg"`
	LastUpdated  int64  `gorm:"column:last_updated;not null" json:"last_updated"`
	
	// Composite unique index on username and area_name
	// This ensures one record per user per area
}

func (LastRead) TableName() string {
	return "lastread"
}

// LastReadConfig represents configuration for lastread database
type LastReadConfig struct {
	DatabasePath string `yaml:"database_path"`
	Enabled      bool   `yaml:"enabled"`
}

// InitLastReadDatabase initializes the separate SQLite database for lastread values
func InitLastReadDatabase(config LastReadConfig) error {
	if !config.Enabled {
		log.Println("Local lastread database disabled")
		return nil
	}

	// Default path if not specified
	dbPath := config.DatabasePath
	if dbPath == "" {
		dbPath = "lastread.db"
	}
	
	// Ensure we have an absolute path
	if !filepath.IsAbs(dbPath) {
		var err error
		dbPath, err = filepath.Abs(dbPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for lastread database: %w", err)
		}
	}

	// Configure GORM logger for lastread database
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Keep it quiet for lastread operations
	}

	var err error
	// Use pure Go SQLite driver (modernc.org/sqlite) - no CGO required
	// This works with CGO_ENABLED=0 unlike the default mattn/go-sqlite3
	LastReadDB, err = gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        dbPath,
	}, gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to lastread database: %w", err)
	}

	// Configure connection pool for SQLite
	sqlDB, err := LastReadDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB for lastread: %w", err)
	}

	// SQLite recommendations
	sqlDB.SetMaxOpenConns(1) // SQLite doesn't benefit from multiple connections
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0) // Keep connections alive

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping lastread database: %w", err)
	}

	// Create table manually (no AutoMigrate)
	if err := LastReadDB.Exec(`
		CREATE TABLE IF NOT EXISTS lastread (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			area_name TEXT NOT NULL,
			last_read_msg INTEGER NOT NULL DEFAULT 0,
			high_read_msg INTEGER NOT NULL DEFAULT 0,
			last_updated INTEGER NOT NULL,
			UNIQUE(username, area_name)
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create lastread table: %w", err)
	}

	log.Printf("Initialized lastread database at %s", dbPath)
	return nil
}

// CloseLastReadDatabase closes the lastread database connection
func CloseLastReadDatabase() error {
	if LastReadDB == nil {
		return nil
	}

	sqlDB, err := LastReadDB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// GetLastRead retrieves the last read position for a user in an area
func GetLastRead(username, areaName string) (uint32, error) {
	if LastReadDB == nil {
		return 0, fmt.Errorf("lastread database not initialized")
	}

	var lastRead LastRead
	err := LastReadDB.Where("username = ? AND area_name = ?", username, areaName).First(&lastRead).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil // No lastread record found, return 0
		}
		return 0, fmt.Errorf("failed to get lastread for user %s in area %s: %w", username, areaName, err)
	}

	return lastRead.LastReadMsg, nil
}

// SetLastRead sets the last read position for a user in an area
func SetLastRead(username, areaName string, position uint32) error {
	if LastReadDB == nil {
		return fmt.Errorf("lastread database not initialized")
	}

	now := time.Now().Unix()
	
	// Use UPSERT (INSERT OR REPLACE for SQLite)
	result := LastReadDB.Exec(`
		INSERT INTO lastread (username, area_name, last_read_msg, high_read_msg, last_updated)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(username, area_name) DO UPDATE SET
			last_read_msg = excluded.last_read_msg,
			high_read_msg = CASE 
				WHEN excluded.high_read_msg > high_read_msg THEN excluded.high_read_msg
				ELSE high_read_msg
			END,
			last_updated = excluded.last_updated
	`, username, areaName, position, position, now)

	if result.Error != nil {
		return fmt.Errorf("failed to set lastread for user %s in area %s: %w", username, areaName, result.Error)
	}

	return nil
}

// GetHighRead retrieves the highest read message for a user in an area
func GetHighRead(username, areaName string) (uint32, error) {
	if LastReadDB == nil {
		return 0, fmt.Errorf("lastread database not initialized")
	}

	var lastRead LastRead
	err := LastReadDB.Where("username = ? AND area_name = ?", username, areaName).First(&lastRead).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil // No lastread record found, return 0
		}
		return 0, fmt.Errorf("failed to get high read for user %s in area %s: %w", username, areaName, err)
	}

	return lastRead.HighReadMsg, nil
}

// GetAllLastReads retrieves all lastread records for a user
func GetAllLastReads(username string) ([]LastRead, error) {
	if LastReadDB == nil {
		return nil, fmt.Errorf("lastread database not initialized")
	}

	var lastReads []LastRead
	err := LastReadDB.Where("username = ?", username).Find(&lastReads).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get all lastreads for user %s: %w", username, err)
	}

	return lastReads, nil
}

// GetLastReadsByArea retrieves all lastread records for an area
func GetLastReadsByArea(areaName string) ([]LastRead, error) {
	if LastReadDB == nil {
		return nil, fmt.Errorf("lastread database not initialized")
	}

	var lastReads []LastRead
	err := LastReadDB.Where("area_name = ?", areaName).Find(&lastReads).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get lastreads for area %s: %w", areaName, err)
	}

	return lastReads, nil
}

// DeleteLastRead removes a lastread record for a user in an area
func DeleteLastRead(username, areaName string) error {
	if LastReadDB == nil {
		return fmt.Errorf("lastread database not initialized")
	}

	result := LastReadDB.Where("username = ? AND area_name = ?", username, areaName).Delete(&LastRead{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete lastread for user %s in area %s: %w", username, areaName, result.Error)
	}

	return nil
}

// DeleteAllLastReadsForUser removes all lastread records for a user
func DeleteAllLastReadsForUser(username string) error {
	if LastReadDB == nil {
		return fmt.Errorf("lastread database not initialized")
	}

	result := LastReadDB.Where("username = ?", username).Delete(&LastRead{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete all lastreads for user %s: %w", username, result.Error)
	}

	log.Printf("Deleted %d lastread records for user %s", result.RowsAffected, username)
	return nil
}

// DeleteAllLastReadsForArea removes all lastread records for an area
func DeleteAllLastReadsForArea(areaName string) error {
	if LastReadDB == nil {
		return fmt.Errorf("lastread database not initialized")
	}

	result := LastReadDB.Where("area_name = ?", areaName).Delete(&LastRead{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete all lastreads for area %s: %w", areaName, result.Error)
	}

	log.Printf("Deleted %d lastread records for area %s", result.RowsAffected, areaName)
	return nil
}

// GetLastReadStats returns statistics about lastread usage
func GetLastReadStats() (map[string]interface{}, error) {
	if LastReadDB == nil {
		return map[string]interface{}{"error": "lastread database not initialized"}, nil
	}

	var totalRecords int64
	var uniqueUsers int64
	var uniqueAreas int64

	// Count total records
	if err := LastReadDB.Model(&LastRead{}).Count(&totalRecords).Error; err != nil {
		return nil, fmt.Errorf("failed to count total lastread records: %w", err)
	}

	// Count unique users
	if err := LastReadDB.Model(&LastRead{}).Distinct("username").Count(&uniqueUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count unique users: %w", err)
	}

	// Count unique areas
	if err := LastReadDB.Model(&LastRead{}).Distinct("area_name").Count(&uniqueAreas).Error; err != nil {
		return nil, fmt.Errorf("failed to count unique areas: %w", err)
	}

	return map[string]interface{}{
		"total_records": totalRecords,
		"unique_users":  uniqueUsers,
		"unique_areas":  uniqueAreas,
	}, nil
}

// IsLastReadEnabled returns true if lastread database is available
func IsLastReadEnabled() bool {
	return LastReadDB != nil
}