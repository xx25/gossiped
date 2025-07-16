package database

import "time"

// EchoAreaType represents the type of message area
type EchoAreaType uint8

const (
	AreaTypeNetmail EchoAreaType = 0 // jnode netmail
	AreaTypeBad     EchoAreaType = 1 // jnode bad
	AreaTypeDupe    EchoAreaType = 2 // jnode dupe
	AreaTypeEcho    EchoAreaType = 3 // jnode echo
	AreaTypeLocal   EchoAreaType = 4 // jnode local
	AreaTypeNone    EchoAreaType = 5 // jnode none
)

// ScheduleType represents script execution frequency
type ScheduleType string

const (
	ScheduleHourly   ScheduleType = "HOURLY"
	ScheduleDaily    ScheduleType = "DAILY"
	ScheduleWeekly   ScheduleType = "WEEKLY"
	ScheduleMonthly  ScheduleType = "MONTHLY"
	ScheduleAnnually ScheduleType = "ANNUALLY"
)

// DateHelper provides utilities for jnode date handling
type DateHelper struct{}

// ToUnixTime converts Go time to Unix timestamp for jnode compatibility
// jnode uses milliseconds since Unix epoch (Java standard)
func (dh DateHelper) ToUnixTime(t time.Time) int64 {
	return t.UnixMilli()
}

// FromUnixTime converts Unix timestamp to Go time
// Handles both seconds and milliseconds automatically
func (dh DateHelper) FromUnixTime(timestamp int64) time.Time {
	// If timestamp is greater than a reasonable seconds value (year 2100),
	// assume it's milliseconds and convert to seconds
	if timestamp > 4102444800 { // Jan 1, 2100 in seconds
		return time.Unix(timestamp/1000, (timestamp%1000)*1000000)
	}
	return time.Unix(timestamp, 0)
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Driver          string        `yaml:"driver"`            // mysql, postgres, sqlite, h2
	DSN             string        `yaml:"dsn"`               // Data Source Name
	MaxOpenConns    int           `yaml:"max_open_conns"`    // Maximum open connections
	MaxIdleConns    int           `yaml:"max_idle_conns"`    // Maximum idle connections
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"` // Connection max lifetime
}

// DefaultDatabaseConfig returns default database configuration
func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Driver:          "sqlite",
		DSN:             "jnode.db",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}
