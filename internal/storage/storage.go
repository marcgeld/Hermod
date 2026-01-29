package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marcgeld/Hermod/internal/logger"
)

// Storage handles database operations
type Storage struct {
	pool      *pgxpool.Pool
	tableName string
	dryRun    bool
	logger    *logger.Logger
}

// Config holds storage configuration
type Config struct {
	ConnectionString string
	TableName        string
	DryRun           bool
	Logger           *logger.Logger
}

var (
	// validTableName ensures table name is safe for SQL
	validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	// validColumnName ensures column name is safe for SQL
	validColumnName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// New creates a new storage instance
func New(ctx context.Context, cfg Config) (*Storage, error) {
	// Validate table name to prevent SQL injection
	if !validTableName.MatchString(cfg.TableName) {
		return nil, fmt.Errorf("invalid table name: must contain only alphanumeric characters and underscores")
	}

	// Use a default logger if none provided
	log := cfg.Logger
	if log == nil {
		log = logger.New(logger.INFO)
	}

	// If dry-run mode, don't connect to database
	if cfg.DryRun {
		log.Info("Storage initialized in dry-run mode (will log SQL instead of executing)")
		return &Storage{
			pool:      nil,
			tableName: cfg.TableName,
			dryRun:    true,
			logger:    log,
		}, nil
	}

	pool, err := pgxpool.New(ctx, cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Storage{
		pool:      pool,
		tableName: cfg.TableName,
		dryRun:    false,
		logger:    log,
	}, nil
}

// Insert inserts a record into the database
func (s *Storage) Insert(ctx context.Context, data map[string]interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data provided")
	}

	// Sort keys to ensure consistent column ordering
	keys := make([]string, 0, len(data))
	for key := range data {
		// Validate column name to prevent SQL injection
		if !validColumnName.MatchString(key) {
			return fmt.Errorf("invalid column name '%s': must contain only alphanumeric characters and underscores", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build the INSERT query with validated identifiers
	columns := make([]string, 0, len(keys))
	placeholders := make([]string, 0, len(keys))
	values := make([]interface{}, 0, len(keys))

	for i, key := range keys {
		columns = append(columns, key)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		
		value := data[key]
		// Convert complex types to JSON
		switch v := value.(type) {
		case map[string]interface{}, []interface{}:
			jsonData, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("failed to marshal %s to JSON: %w", key, err)
			}
			values = append(values, jsonData)
		default:
			values = append(values, value)
		}
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		s.tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// In dry-run mode, just log the SQL
	if s.dryRun {
		s.logger.Infof("SQL (dry-run): %s", query)
		s.logger.Debugf("SQL Values: %v", values)
		return nil
	}

	_, err := s.pool.Exec(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}

	return nil
}

// Close closes the database connection pool
func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
