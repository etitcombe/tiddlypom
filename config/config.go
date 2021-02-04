package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// DbConfig represents the configuration settings for the database.
type DbConfig struct {
	Host     string `json:"host"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	Port     int    `json:"port"`
}

// Config represents the configuration settings of the application.
type Config struct {
	Pepper   string   `json:"pepper"`
	Port     int      `json:"port"`
	Database DbConfig `json:"database"`
}

// LoadConfig loads the configuration from .config.
func LoadConfig() (Config, error) {
	f, err := os.Open(".config")
	if err != nil {
		return Config{}, fmt.Errorf("error opening .config: %w", err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return Config{}, fmt.Errorf("error reading .config: %w", err)
	}

	var c Config
	err = json.Unmarshal(data, &c)
	if err != nil {
		return Config{}, fmt.Errorf("error unmarshalling .config: %w", err)
	}
	return c, nil
}
