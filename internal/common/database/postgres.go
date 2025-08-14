// internal/common/database/postgres.go
// PostgreSQL connection and configuration

package database

import (
    "database/sql"
    "fmt"
    "time"
    
    _ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresConfig holds database configuration
type PostgresConfig struct {
    Host         string
    Port         int
    User         string
    Password     string
    Database     string
    SSLMode      string
    MaxOpenConns int
    MaxIdleConns int
    MaxLifetime  time.Duration
}

// NewPostgresDB creates a new PostgreSQL connection
func NewPostgresDB(config *PostgresConfig) (*sql.DB, error) {
    // Build connection string
    dsn := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        config.Host,
        config.Port,
        config.User,
        config.Password,
        config.Database,
        config.SSLMode,
    )
    
    // Open connection
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Configure connection pool
    db.SetMaxOpenConns(config.MaxOpenConns)
    db.SetMaxIdleConns(config.MaxIdleConns)
    db.SetConnMaxLifetime(config.MaxLifetime)
    
    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    return db, nil
}

// NewPostgresDBFromURL creates a connection from a URL
func NewPostgresDBFromURL(databaseURL string) (*sql.DB, error) {
    db, err := sql.Open("postgres", databaseURL)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Configure connection pool with defaults
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    return db, nil
}

