package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"db_uploader/internal/models"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type PostgresDB struct {
	pool          *pgxpool.Pool
	table         pgx.Identifier
	insertedCount int64
}

type PostgresConfig struct {
	DSN             string
	Table           string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func NewPostgresDB(dsn string, table string) (*PostgresDB, error) {
	return NewPostgresDBWithConfig(PostgresConfig{
		DSN:   dsn,
		Table: table,
	})
}

func NewPostgresDBWithConfig(config PostgresConfig) (*PostgresDB, error) {
	if strings.TrimSpace(config.DSN) == "" {
		return nil, fmt.Errorf("dsn is required for postgres driver")
	}

	validatedTable, err := validateTableName(config.Table)
	if err != nil {
		return nil, err
	}

	poolConfig, err := pgxpool.ParseConfig(config.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	if config.MaxOpenConns > 0 {
		poolConfig.MaxConns = int32(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		poolConfig.MinConns = min(int32(config.MaxIdleConns), poolConfig.MaxConns)
	}
	if config.ConnMaxLifetime > 0 {
		poolConfig.MaxConnLifetime = config.ConnMaxLifetime
	}
	if config.ConnMaxIdleTime > 0 {
		poolConfig.MaxConnIdleTime = config.ConnMaxIdleTime
	}
	// Prefer simple protocol semantics through poolers and let COPY use the wire protocol directly.
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctxBackground(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	if err := pool.Ping(ctxBackground()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresDB{
		pool:  pool,
		table: validatedTable,
	}, nil
}

func (db *PostgresDB) Close() error {
	db.pool.Close()
	return nil
}

func (db *PostgresDB) InsertBatch(users []models.User) error {
	if len(users) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(users))
	for _, user := range users {
		createdAt, err := time.Parse(time.RFC3339, user.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse created_at for user %d: %w", user.ID, err)
		}

		rows = append(rows, []any{
			user.ID,
			user.Name,
			user.Email,
			user.Age,
			user.City,
			createdAt,
		})
	}

	if _, err := db.pool.CopyFrom(
		ctxBackground(),
		db.table,
		[]string{"id", "name", "email", "age", "city", "created_at"},
		pgx.CopyFromRows(rows),
	); err != nil {
		return err
	}

	atomic.AddInt64(&db.insertedCount, int64(len(users)))
	return nil
}

func (db *PostgresDB) GetTotalInserted() int64 {
	return atomic.LoadInt64(&db.insertedCount)
}

func (db *PostgresDB) GetPoolStats() PoolStats {
	stats := db.pool.Stat()
	return PoolStats{
		MaxConns:                stats.MaxConns(),
		AcquiredConns:           stats.AcquiredConns(),
		IdleConns:               stats.IdleConns(),
		TotalConns:              stats.TotalConns(),
		AcquireCount:            stats.AcquireCount(),
		AcquireDuration:         stats.AcquireDuration(),
		CanceledAcquireCount:    stats.CanceledAcquireCount(),
		EmptyAcquireCount:       stats.EmptyAcquireCount(),
		NewConnsCount:           stats.NewConnsCount(),
		MaxLifetimeDestroyCount: stats.MaxLifetimeDestroyCount(),
		MaxIdleDestroyCount:     stats.MaxIdleDestroyCount(),
	}
}

func (db *PostgresDB) EnsureBenchmarkTable() error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGINT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		age INT NOT NULL,
		city TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	)`, db.table.Sanitize())

	_, err := db.pool.Exec(ctxBackground(), query)
	return err
}

func (db *PostgresDB) Truncate() error {
	_, err := db.pool.Exec(ctxBackground(), "TRUNCATE TABLE "+db.table.Sanitize())
	return err
}

func validateTableName(table string) (pgx.Identifier, error) {
	t := strings.TrimSpace(table)
	if t == "" {
		return nil, fmt.Errorf("table name is required")
	}

	parts := strings.Split(t, ".")
	for _, part := range parts {
		if !identifierPattern.MatchString(part) {
			return nil, fmt.Errorf("invalid table name %q", table)
		}
	}

	return pgx.Identifier(parts), nil
}

type PoolStats struct {
	MaxConns                int32         `json:"maxConns"`
	AcquiredConns           int32         `json:"acquiredConns"`
	IdleConns               int32         `json:"idleConns"`
	TotalConns              int32         `json:"totalConns"`
	AcquireCount            int64         `json:"acquireCount"`
	AcquireDuration         time.Duration `json:"acquireDuration"`
	CanceledAcquireCount    int64         `json:"canceledAcquireCount"`
	EmptyAcquireCount       int64         `json:"emptyAcquireCount"`
	NewConnsCount           int64         `json:"newConnsCount"`
	MaxLifetimeDestroyCount int64         `json:"maxLifetimeDestroyCount"`
	MaxIdleDestroyCount     int64         `json:"maxIdleDestroyCount"`
}

func ctxBackground() context.Context {
	return context.Background()
}

func min(a int32, b int32) int32 {
	if b == 0 || a < b {
		return a
	}
	return b
}
