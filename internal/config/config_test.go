package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid config",
			content: `
[mqtt]
broker = "tcp://localhost:1883"
client_id = "test-client"
username = "user"
password = "pass"
topics = ["test/#"]
qos = 1

[database]
host = "localhost"
port = 5432
user = "testuser"
password = "testpass"
database = "testdb"
sslmode = "disable"
pool_size = 10

[pipeline]
lua_script = "script.lua"
table_name = "test_table"
`,
			wantErr: false,
		},
		{
			name:    "invalid toml syntax",
			content: `[mqtt\nbroker = invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.toml")
			
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg == nil {
					t.Error("Load() returned nil config without error")
					return
				}

				// Verify config was parsed correctly
				if cfg.MQTT.Broker != "tcp://localhost:1883" {
					t.Errorf("MQTT.Broker = %v, want tcp://localhost:1883", cfg.MQTT.Broker)
				}
				if cfg.MQTT.ClientID != "test-client" {
					t.Errorf("MQTT.ClientID = %v, want test-client", cfg.MQTT.ClientID)
				}
				if cfg.Database.Host != "localhost" {
					t.Errorf("Database.Host = %v, want localhost", cfg.Database.Host)
				}
				if cfg.Database.Port != 5432 {
					t.Errorf("Database.Port = %v, want 5432", cfg.Database.Port)
				}
				if cfg.Pipeline.TableName != "test_table" {
					t.Errorf("Pipeline.TableName = %v, want test_table", cfg.Pipeline.TableName)
				}
			}
		})
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestDatabaseConfigConnectionString(t *testing.T) {
	tests := []struct {
		name   string
		config DatabaseConfig
		want   string
	}{
		{
			name: "basic connection string",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
				SSLMode:  "disable",
				PoolSize: 10,
			},
			want: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable pool_max_conns=10",
		},
		{
			name: "connection string with ssl enabled",
			config: DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				User:     "admin",
				Password: "secret123",
				Database: "proddb",
				SSLMode:  "require",
				PoolSize: 20,
			},
			want: "host=db.example.com port=5433 user=admin password=secret123 dbname=proddb sslmode=require pool_max_conns=20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ConnectionString()
			if got != tt.want {
				t.Errorf("ConnectionString() = %v, want %v", got, tt.want)
			}
		})
	}
}
