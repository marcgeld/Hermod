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
	Routes   []RouteConfig  `toml:"routes"` // New routing configuration
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

// RouteConfig holds a single route configuration
type RouteConfig struct {
	Filter    string `toml:"filter"`     // MQTT topic filter (e.g., "ruuvi/+", "p1ib/#")
	Script    string `toml:"script"`     // Path to Lua script (empty = passthrough)
	Workers   int    `toml:"workers"`    // Number of worker goroutines (default: 1)
	QueueSize int    `toml:"queue_size"` // Buffered channel size (default: 100)
	Table     string `toml:"table"`      // Default table name (default: iot_data)
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
