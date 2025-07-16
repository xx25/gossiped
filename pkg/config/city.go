package config

import (
	"fmt"
	"os"

	"github.com/askovpen/gossiped/pkg/types"
	"gopkg.in/yaml.v3"
)

func readCity() error {
	yamlFile, err := os.ReadFile(Config.CityPath)
	if err != nil {
		return fmt.Errorf("cannot read city file '%s': %w", Config.CityPath, err)
	}
	err = yaml.Unmarshal(yamlFile, &city)
	if err != nil {
		return err
	}
	return nil
}

// GetCity return city
func GetCity(sfa *types.FidoAddr) string {
	if val, ok := city[sfa.ShortString()]; ok {
		return val
	}
	return "unknown"
}
