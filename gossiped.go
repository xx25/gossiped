package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/askovpen/gossiped/pkg/areasconfig"
	"github.com/askovpen/gossiped/pkg/config"
	"github.com/askovpen/gossiped/pkg/database"
	"github.com/askovpen/gossiped/pkg/ui"
	"github.com/askovpen/gossiped/pkg/utils"
)

var (
	version = "2.1"
	commit  = "dev"
)

func tryFindConfig() string {
	for _, fn := range []string{
		filepath.Join(os.Getenv("HOME"), "gossiped.yml"),
		filepath.Join(os.Getenv("HOME"), ".config", "gossiped.yml"),
		"/usr/local/etc/ftn/gossiped.yml",
		"/etc/ftn/gossiped.yml",
		"gossiped.yml",
	} {
		if utils.FileExists(fn) {
			return fn
		}
	}
	return ""
}

// setupGracefulShutdown sets up signal handlers for graceful shutdown
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Print("Shutdown signal received, cleaning up...")

		// Close database connections
		if isUsingSQLAreas() {
			log.Print("Closing database connection...")
			if err := database.CloseDatabase(); err != nil {
				log.Printf("Error closing database during shutdown: %v", err)
			}
		}
		
		// Close lastread database if enabled
		if database.IsLastReadEnabled() {
			log.Print("Closing lastread database...")
			if err := database.CloseLastReadDatabase(); err != nil {
				log.Printf("Error closing lastread database during shutdown: %v", err)
			}
		}

		log.Print("Graceful shutdown complete")
		os.Exit(0)
	}()
}

// isUsingSQLAreas returns true if the application is configured to use SQL areas
func isUsingSQLAreas() bool {
	return config.Config.AreaFile.Type == "jnode-sql"
}

// logStartupInfo logs startup information about the current configuration
func logStartupInfo() {
	log.Printf("gossiped %s started", config.LongPID)
	log.Printf("Area configuration type: %s", config.Config.AreaFile.Type)

	if isUsingSQLAreas() {
		dbConfig := config.GetDatabaseConfig()
		log.Printf("Database driver: %s", dbConfig.Driver)
		log.Printf("Database DSN: %s", maskPassword(dbConfig.DSN))
		log.Printf("Connection pool - Max open: %d, Max idle: %d, Max lifetime: %v",
			dbConfig.MaxOpenConns, dbConfig.MaxIdleConns, dbConfig.ConnMaxLifetime)
	} else {
		log.Printf("Area file path: %s", config.Config.AreaFile.Path)
	}
}

// maskPassword masks sensitive information in DSN strings for logging
func maskPassword(dsn string) string {
	// Simple masking for common DSN formats
	if len(dsn) > 20 {
		// For MySQL/PostgreSQL connection strings with passwords
		if pos := strings.Index(dsn, ":"); pos > 0 {
			if pos2 := strings.Index(dsn[pos+1:], "@"); pos2 > 0 {
				return dsn[:pos+1] + "***" + dsn[pos+1+pos2:]
			}
		}
		// For long SQLite paths, show only filename
		if strings.HasSuffix(dsn, ".db") {
			return ".../" + filepath.Base(dsn)
		}
	}
	return dsn
}

func main() {
	if len(commit) > 8 {
		commit = commit[0:8]
	}
	if commit == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					commit += "-" + setting.Value[0:8]
				}
			}
		}
	}
	config.Version = version + "-" + commit
	config.InitVars()
	var fn string
	if len(os.Args) == 1 {
		fn = tryFindConfig()
		if fn == "" {
			log.Printf("Usage: %s <config.yml>", os.Args[0])
			return
		}
	} else {
		if utils.FileExists(os.Args[1]) {
			fn = os.Args[1]
		} else {
			log.Printf("Usage: %s <config.yml>", os.Args[0])
			return
		}
	}
	log.Printf("reading configuration from %s\n", fn)
	err := config.Read(fn)
	if err != nil {
		log.Println(err)
		return
	}
	f, _ := os.OpenFile(config.Config.Log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	defer f.Close()
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Log startup information
	logStartupInfo()

	// Setup graceful shutdown for database cleanup
	setupGracefulShutdown()

	// Initialize lastread database if enabled
	lastReadConfig := config.GetLastReadConfig()
	if lastReadConfig.Enabled {
		log.Print("Initializing lastread database")
		err = database.InitLastReadDatabase(lastReadConfig)
		if err != nil {
			log.Printf("Error initializing lastread database: %v", err)
			// Continue without lastread functionality
		}
	}

	log.Print("reading areas")
	err = areasconfig.Read()
	if err != nil {
		log.Print(err)
		// Clean up database connections if they were initialized
		if isUsingSQLAreas() {
			database.CloseDatabase()
		}
		if database.IsLastReadEnabled() {
			database.CloseLastReadDatabase()
		}
		return
	}

	// Perform database health check for SQL areas
	if isUsingSQLAreas() {
		if err := database.HealthCheck(); err != nil {
			log.Printf("Database health check failed: %v", err)
			database.CloseDatabase()
			if database.IsLastReadEnabled() {
				database.CloseLastReadDatabase()
			}
			return
		}
		log.Print("Database connection healthy")

		// Display database connection statistics
		stats := database.GetConnectionStats()
		if errVal, hasErr := stats["error"]; hasErr {
			log.Printf("Warning: Could not get database stats: %v", errVal)
		} else {
			log.Printf("Database connection pool - Open: %v, InUse: %v, Idle: %v",
				stats["open_connections"], stats["in_use"], stats["idle"])
		}
	}

	log.Print("starting ui")
	app := ui.NewApp()
	if err = app.Run(); err != nil {
		log.Print("UI error occurred")
		log.Print(err)
		// Clean up database connections on error
		if isUsingSQLAreas() {
			database.CloseDatabase()
		}
		if database.IsLastReadEnabled() {
			database.CloseLastReadDatabase()
		}
		return
	}

	// Clean up database connections on normal exit
	if isUsingSQLAreas() {
		log.Print("Closing database connection")
		if err := database.CloseDatabase(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
	
	// Close lastread database if enabled
	if database.IsLastReadEnabled() {
		log.Print("Closing lastread database")
		if err := database.CloseLastReadDatabase(); err != nil {
			log.Printf("Error closing lastread database: %v", err)
		}
	}

	log.Print("gossiped shutdown complete")
}
