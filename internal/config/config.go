package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config represents the application configuration
type Config struct {
	MQTT     MQTTConfig     `toml:"mqtt"`
	Database DatabaseConfig `toml:"database"`
	Pipeline PipelineConfig `toml:"pipeline"`
	Logging  LoggingConfig  `toml:"logging"`
}

// MQTTConfig holds MQTT broker configuration
type MQTTConfig struct {
	Broker   string   `toml:"broker"`
	ClientID string   `toml:"client_id"`
	Username string   `toml:"username"`
	Password string   `toml:"password"`
	Topics   []string `toml:"topics"`
	QoS      byte     `toml:"qos"`
}

// DatabaseConfig holds PostgreSQL/TimescaleDB configuration
type DatabaseConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	SSLMode  string `toml:"sslmode"`
	PoolSize int    `toml:"pool_size"`
}

// PipelineConfig holds pipeline configuration
type PipelineConfig struct {
	LuaScript string `toml:"lua_script"`
	TableName string `toml:"table_name"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level string `toml:"level"` // DEBUG, INFO, or ERROR
}

// Load reads and parses the TOML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// ConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d",
		d.Host, d.Port, d.User, d.Password, d.Database, d.SSLMode, d.PoolSize,
	)
}
