package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	_ "github.com/lib/pq"

	"db_uploader/internal/models"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type PostgresDB struct {
	conn          *sql.DB
	table         string
	insertedCount int64
}

func NewPostgresDB(dsn string, table string) (*PostgresDB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("dsn is required for postgres driver")
	}

	validatedTable, err := validateTableName(table)
	if err != nil {
		return nil, err
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
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
