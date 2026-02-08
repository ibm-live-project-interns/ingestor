// Package database provides a shared GORM database connection
// for all services in the ingestor ecosystem.
package database

import (
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// DBConfig holds database configuration
type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	LogLevel        gormlogger.LogLevel
}

// DefaultDBConfig returns configuration from environment variables
func DefaultDBConfig() DBConfig {
	logLevel := gormlogger.Info
	if config.GetEnv("GIN_MODE", "debug") == "release" {
		logLevel = gormlogger.Error
	}

	return DBConfig{
		Host:            config.GetEnv("POSTGRES_HOST", "localhost"),
		Port:            config.GetEnv("POSTGRES_PORT", "5432"),
		User:            config.GetEnv("POSTGRES_USER", "postgres"),
		Password:        config.GetEnv("POSTGRES_PASSWORD", ""),
		DBName:          config.GetEnv("POSTGRES_DB", "noc_alerts"),
		SSLMode:         config.GetEnv("POSTGRES_SSLMODE", "disable"),
		MaxIdleConns:    config.GetEnvInt("DB_MAX_IDLE_CONNS", 10),
		MaxOpenConns:    config.GetEnvInt("DB_MAX_OPEN_CONNS", 100),
		ConnMaxLifetime: time.Duration(config.GetEnvInt("DB_CONN_MAX_LIFETIME_MINS", 60)) * time.Minute,
		LogLevel:        logLevel,
	}
}

// DSN returns the PostgreSQL connection string
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// Validate checks if required configuration is present
func (c DBConfig) Validate() error {
	if c.Password == "" {
		return fmt.Errorf("POSTGRES_PASSWORD environment variable is required")
	}
	if c.Host == "" {
		return fmt.Errorf("POSTGRES_HOST is required")
	}
	return nil
}

// Database wraps the GORM DB instance with additional methods
type Database struct {
	*gorm.DB
	config DBConfig
}

var (
	instance *Database
	once     sync.Once
	initErr  error
)

// Init initializes the global database instance
// Should be called once at application startup
func Init(cfg DBConfig) (*Database, error) {
	once.Do(func() {
		instance, initErr = NewDatabase(cfg)
	})
	return instance, initErr
}

// InitWithDefaults initializes with default configuration from environment
func InitWithDefaults() (*Database, error) {
	return Init(DefaultDBConfig())
}

// Get returns the global database instance
// Returns nil if Init was not called or failed
func Get() *Database {
	return instance
}

// NewDatabase creates a new database connection
func NewDatabase(cfg DBConfig) (*Database, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	logger.Info("Connecting to database: %s@%s:%s/%s", cfg.User, cfg.Host, cfg.Port, cfg.DBName)

	// Configure GORM logger
	gormLogger := gormlogger.Default.LogMode(cfg.LogLevel)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	logger.Info("Database connected successfully")

	return &Database{
		DB:     db,
		config: cfg,
	}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	logger.Info("Closing database connection")
	return sqlDB.Close()
}

// Ping checks database connectivity
func (d *Database) Ping() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// AutoMigrate runs migrations for the given models
func (d *Database) AutoMigrate(models ...interface{}) error {
	logger.Info("Running database migrations...")
	if err := d.DB.AutoMigrate(models...); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	logger.Info("Database migrations completed")
	return nil
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(fn func(tx *gorm.DB) error) error {
	return d.DB.Transaction(fn)
}

// Close closes the global database instance
func Close() error {
	if instance != nil {
		return instance.Close()
	}
	return nil
}
