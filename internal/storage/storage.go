package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage handles database operations
type Storage struct {
	pool      *pgxpool.Pool
	tableName string
}

// Config holds storage configuration
type Config struct {
	ConnectionString string
	TableName        string
}

// New creates a new storage instance
func New(ctx context.Context, cfg Config) (*Storage, error) {
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
	}, nil
}

// Insert inserts a record into the database
func (s *Storage) Insert(ctx context.Context, data map[string]interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data provided")
	}

	// Build the INSERT query dynamically
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))
	i := 1

	for key, value := range data {
		columns = append(columns, key)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))

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
		i++
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		s.tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

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
