package areasconfig

import (
	"fmt"
	"log"

	"github.com/askovpen/gossiped/pkg/config"
	"github.com/askovpen/gossiped/pkg/database"
	"github.com/askovpen/gossiped/pkg/msgapi"
	"gorm.io/gorm"
)

// jnodeConfigRead loads areas from jnode SQL database
func jnodeConfigRead() error {
	// Get database configuration
	dbConfig := config.GetDatabaseConfig()

	// Initialize database connection
	err := database.InitDatabase(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	db := database.GetDatabase()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	log.Printf("Connected to jnode database, loading areas...")

	// Load echoareas from database
	err = loadEchoareas(db)
	if err != nil {
		return fmt.Errorf("failed to load echoareas: %w", err)
	}

	// Add netmail area
	err = loadNetmailArea(db)
	if err != nil {
		return fmt.Errorf("failed to load netmail area: %w", err)
	}

	log.Printf("Loaded %d areas from jnode database", len(msgapi.Areas))
	return nil
}

// loadEchoareas loads echo areas from the database
func loadEchoareas(db *gorm.DB) error {
	var echoareas []database.Echoarea

	// Load all echoareas from database
	err := db.Find(&echoareas).Error
	if err != nil {
		return fmt.Errorf("error querying echoareas: %w", err)
	}

	// Refresh message counts cache for all areas at once
	log.Printf("Loading message counts for %d echoareas...", len(echoareas))
	if err := msgapi.RefreshMessageCounts(); err != nil {
		log.Printf("Warning: Failed to load message counts cache: %v", err)
		// Continue without cache - fallback to individual queries
	}

	for _, echoarea := range echoareas {
		// Create SQL area instance
		sqlArea := msgapi.NewSQLArea(db, echoarea)

		// Apply character set from configuration if specified
		if charset := findAreaCharset(echoarea.Name); charset != "" {
			sqlArea.SetChrs(charset)
		}

		// Initialize the area
		sqlArea.Init()

		// Add to global areas list
		msgapi.Areas = append(msgapi.Areas, sqlArea)

		log.Printf("Loaded echoarea: %s (%s)", echoarea.Name, echoarea.Description)
	}

	return nil
}

// loadNetmailArea creates and loads the netmail area
func loadNetmailArea(db *gorm.DB) error {
	// Create netmail area
	netmailArea := msgapi.NewSQLNetmailArea(db)

	// Apply character set from configuration if specified
	if charset := findAreaCharset("Netmail"); charset != "" {
		netmailArea.SetChrs(charset)
	}

	// Initialize the area
	netmailArea.Init()

	// Add to global areas list
	msgapi.Areas = append(msgapi.Areas, netmailArea)

	log.Printf("Loaded netmail area")
	return nil
}

// findAreaCharset finds character set for an area from configuration
func findAreaCharset(areaName string) string {
	for _, configArea := range config.Config.Areas {
		if configArea.Name == areaName && configArea.Chrs != "" {
			return configArea.Chrs
		}
	}
	return ""
}

// loadSubscribedAreas loads only areas that the configured node is subscribed to
func loadSubscribedAreas(db *gorm.DB, nodeAddress string) error {
	// First, find the link ID for our node address
	var link database.Link
	err := db.Where("ftn_address = ?", nodeAddress).First(&link).Error
	if err != nil {
		// If our node is not in the links table, load all areas
		log.Printf("Node %s not found in links table, loading all areas", nodeAddress)
		return loadEchoareas(db)
	}

	// Load subscribed echoareas
	var subscriptions []database.Subscription
	err = db.Where("link_id = ?", link.ID).Preload("Echoarea").Find(&subscriptions).Error
	if err != nil {
		return fmt.Errorf("error querying subscriptions: %w", err)
	}

	for _, subscription := range subscriptions {
		// Create SQL area instance
		sqlArea := msgapi.NewSQLArea(db, subscription.Echoarea)

		// Apply character set from configuration if specified
		if charset := findAreaCharset(subscription.Echoarea.Name); charset != "" {
			sqlArea.SetChrs(charset)
		}

		// Initialize the area
		sqlArea.Init()

		// Add to global areas list
		msgapi.Areas = append(msgapi.Areas, sqlArea)

		log.Printf("Loaded subscribed echoarea: %s (%s)",
			subscription.Echoarea.Name, subscription.Echoarea.Description)
	}

	return nil
}

// GetDatabaseConnection returns the current database connection
// This can be used by other parts of the application
func GetDatabaseConnection() *gorm.DB {
	return database.GetDatabase()
}

// RefreshAreas reloads areas from the database
// Useful for runtime area management
func RefreshAreas() error {
	// Clear current areas
	msgapi.Areas = nil

	// Reload from database
	return jnodeConfigRead()
}

// GetAreaByName finds an area by name in the loaded areas
func GetAreaByName(name string) msgapi.AreaPrimitive {
	for _, area := range msgapi.Areas {
		if area.GetName() == name {
			return area
		}
	}
	return nil
}

// GetEchoareaInfo returns database information about an echoarea
func GetEchoareaInfo(areaName string) (*database.Echoarea, error) {
	db := database.GetDatabase()
	if db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	var echoarea database.Echoarea
	err := db.Where("name = ?", areaName).First(&echoarea).Error
	if err != nil {
		return nil, fmt.Errorf("echoarea %s not found: %w", areaName, err)
	}

	return &echoarea, nil
}

// CreateEchoarea creates a new echoarea in the database
func CreateEchoarea(name, description string, readLevel, writeLevel int64, group string) error {
	db := database.GetDatabase()
	if db == nil {
		return fmt.Errorf("database connection not available")
	}

	echoarea := database.Echoarea{
		Name:        name,
		Description: description,
		RLevel:      readLevel,
		WLevel:      writeLevel,
		Grp:         group,
	}

	err := db.Create(&echoarea).Error
	if err != nil {
		return fmt.Errorf("failed to create echoarea %s: %w", name, err)
	}

	log.Printf("Created new echoarea: %s", name)
	return nil
}

// DeleteEchoarea removes an echoarea from the database
func DeleteEchoarea(areaName string) error {
	db := database.GetDatabase()
	if db == nil {
		return fmt.Errorf("database connection not available")
	}

	// First, delete all messages in the area
	var echoarea database.Echoarea
	err := db.Where("name = ?", areaName).First(&echoarea).Error
	if err != nil {
		return fmt.Errorf("echoarea %s not found: %w", areaName, err)
	}

	// Delete messages
	err = db.Where("echoarea_id = ?", echoarea.ID).Delete(&database.Echomail{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete messages in area %s: %w", areaName, err)
	}

	// Delete subscriptions
	err = db.Where("echoarea_id = ?", echoarea.ID).Delete(&database.Subscription{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete subscriptions for area %s: %w", areaName, err)
	}

	// Delete the echoarea itself
	err = db.Delete(&echoarea).Error
	if err != nil {
		return fmt.Errorf("failed to delete echoarea %s: %w", areaName, err)
	}

	log.Printf("Deleted echoarea: %s", areaName)
	return nil
}

// GetAreaStatistics returns statistics for all areas
func GetAreaStatistics() map[string]int64 {
	db := database.GetDatabase()
	if db == nil {
		return nil
	}

	stats := make(map[string]int64)

	// Get echomail statistics
	var echoareas []database.Echoarea
	db.Find(&echoareas)

	for _, echoarea := range echoareas {
		var count int64
		db.Model(&database.Echomail{}).Where("echoarea_id = ?", echoarea.ID).Count(&count)
		stats[echoarea.Name] = count
	}

	// Get netmail statistics
	var netmailCount int64
	db.Model(&database.Netmail{}).Count(&netmailCount)
	stats["Netmail"] = netmailCount

	return stats
}

// HealthCheck performs a health check on the database connection
func HealthCheck() error {
	return database.HealthCheck()
}
