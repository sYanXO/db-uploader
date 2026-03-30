package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"

	"db_uploader/internal/models"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type PostgresDB struct {
	conn          *sql.DB
	table         string
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

	conn, err := sql.Open("postgres", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if config.MaxOpenConns > 0 {
		conn.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		conn.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		conn.SetConnMaxLifetime(config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime > 0 {
		conn.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresDB{
		conn:  conn,
		table: validatedTable,
	}, nil
}

func (db *PostgresDB) Close() error {
	return db.conn.Close()
}

func (db *PostgresDB) InsertBatch(users []models.User) error {
	if len(users) == 0 {
		return nil
	}

	query, args := buildInsertQuery(db.table, users)
	if _, err := db.conn.Exec(query, args...); err != nil {
		return err
	}

	atomic.AddInt64(&db.insertedCount, int64(len(users)))
	return nil
}

func (db *PostgresDB) GetTotalInserted() int64 {
	return atomic.LoadInt64(&db.insertedCount)
}

func (db *PostgresDB) GetSQLStats() sql.DBStats {
	return db.conn.Stats()
}

func (db *PostgresDB) EnsureBenchmarkTable() error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGINT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		age INT NOT NULL,
		city TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	)`, db.table)

	_, err := db.conn.Exec(query)
	return err
}

func (db *PostgresDB) Truncate() error {
	_, err := db.conn.Exec("TRUNCATE TABLE " + db.table)
	return err
}

func buildInsertQuery(table string, users []models.User) (string, []any) {
	const fieldsPerRow = 6

	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	builder.WriteString(table)
	builder.WriteString(" (id, name, email, age, city, created_at) VALUES ")

	args := make([]any, 0, len(users)*fieldsPerRow)
	for i, user := range users {
		if i > 0 {
			builder.WriteString(", ")
		}

		base := i * fieldsPerRow
		builder.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", base+1, base+2, base+3, base+4, base+5, base+6))
		args = append(args, user.ID, user.Name, user.Email, user.Age, user.City, user.CreatedAt)
	}

	return builder.String(), args
}

func validateTableName(table string) (string, error) {
	t := strings.TrimSpace(table)
	if t == "" {
		return "", fmt.Errorf("table name is required")
	}

	parts := strings.Split(t, ".")
	for _, part := range parts {
		if !identifierPattern.MatchString(part) {
			return "", fmt.Errorf("invalid table name %q", table)
		}
	}

	return t, nil
}
