package config

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/askovpen/gossiped/pkg/database"
	"github.com/askovpen/gossiped/pkg/nodelist"
	"github.com/askovpen/gossiped/pkg/types"
	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"
)

type (
	ColorMap    map[string]string
	SortTypeMap map[string]string
	configS     struct {
		Username string
		AreaFile struct {
			Path string
			Type string
		}
		Areas []struct {
			Name     string
			Path     string
			Type     string
			BaseType string
			Chrs     string
		}
		Database struct {
			Driver          string        `yaml:"driver"`
			DSN             string        `yaml:"dsn"`
			MaxOpenConns    int           `yaml:"max_open_conns"`
			MaxIdleConns    int           `yaml:"max_idle_conns"`
			ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
		}
		LastRead struct {
			Enabled      bool   `yaml:"enabled"`
			DatabasePath string `yaml:"database_path"`
		}
		Colorscheme string
		Log         string
		Address     *types.FidoAddr
		Origin      string
		Tearline    string
		Template    string
		Chrs        struct {
			Default      string
			IBMPC        string
			JnodeDefault string
		}
		Statusbar struct {
			Clock bool
		}
		Sorting      SortTypeMap
		Colors       map[string]ColorMap
		CityPath     string
		NodelistPath string
	}
)

// vars
var (
	Version      string
	PID          string
	LongPID      string
	Config       configS
	Template     []string
	city         map[string]string
	StyleDefault tcell.Style
)

// InitVars define version variables
func InitVars() {
	PID = "gossipEd+" + runtime.GOOS[0:3] + " " + Version
	LongPID = "gossipEd-" + runtime.GOOS + "/" + runtime.GOARCH + " " + Version
}
func tryPath(rootPath string, filePath string) string {
	if _, err := os.Stat(filePath); err == nil {
		return filePath
	}
	if _, err := os.Stat(path.Join(rootPath, filePath)); err == nil {
		return path.Join(rootPath, filePath)
	}
	return ""
}

// Read config
func Read(fn string) error {
	yamlFile, err := os.ReadFile(fn)
	if err != nil {
		return err
	}
	rootPath := filepath.Dir(fn)

	err = yaml.Unmarshal(yamlFile, &Config)
	if err != nil {
		return err
	}
	if Config.Address == nil {
		return errors.New("Config.Address not defined")
	}
	if Config.Chrs.Default == "" {
		return errors.New("Config.Chrs.Default not defined")
	}
	Config.Template = tryPath(rootPath, Config.Template)
	tpl, err := os.ReadFile(Config.Template)
	if err != nil {
		return err
	}
	readTemplate(tpl)
	if len(Config.Tearline) == 0 {
		Config.Tearline = LongPID
	}
	errColors := readColors(rootPath)
	if errColors != nil {
		return errColors
	}
	if Config.CityPath == "" {
		return errors.New("Config.CityPath not defined")
	}
	Config.CityPath = tryPath(rootPath, Config.CityPath)
	err = readCity()
	Config.NodelistPath = tryPath(rootPath, Config.NodelistPath)
	nodelist.Read(Config.NodelistPath)
	if err != nil {
		return err
	}
	// Set database defaults if not specified
	setDatabaseDefaults()

	return nil
}

// setDatabaseDefaults sets default values for database configuration
func setDatabaseDefaults() {
	if Config.Database.Driver == "" {
		Config.Database.Driver = "sqlite"
	}
	if Config.Database.DSN == "" {
		Config.Database.DSN = "jnode.db"
	}
	if Config.Database.MaxOpenConns == 0 {
		Config.Database.MaxOpenConns = 25
	}
	if Config.Database.MaxIdleConns == 0 {
		Config.Database.MaxIdleConns = 5
	}
	if Config.Database.ConnMaxLifetime == 0 {
		Config.Database.ConnMaxLifetime = 5 * time.Minute
	}
}

// GetDatabaseConfig returns the database configuration with defaults applied
func GetDatabaseConfig() database.DatabaseConfig {
	return database.DatabaseConfig{
		Driver:          Config.Database.Driver,
		DSN:             Config.Database.DSN,
		MaxOpenConns:    Config.Database.MaxOpenConns,
		MaxIdleConns:    Config.Database.MaxIdleConns,
		ConnMaxLifetime: Config.Database.ConnMaxLifetime,
	}
}

// GetLastReadConfig returns the lastread configuration with defaults applied
func GetLastReadConfig() database.LastReadConfig {
	return database.LastReadConfig{
		Enabled:      Config.LastRead.Enabled,
		DatabasePath: Config.LastRead.DatabasePath,
	}
}

func readTemplate(tpl []byte) {
	for _, l := range strings.Split(string(tpl), "\n") {
		if len(l) > 0 && l[0] == ';' {
			continue
		}
		Template = append(Template, l)
	}
}
