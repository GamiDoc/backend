package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	sql *sql.DB
}

// New opens the pool with pgx in simple protocol: no prepared statements, so it also works
// behind a pooler in transaction mode (Supabase, port 6543), where prepared statements land
// on different server connections ("unnamed prepared statement does not exist"). JSON
// parameters are passed as strings: in simple protocol []byte would be sent as bytea.
func New(dsn string) (*DB, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db := stdlib.OpenDB(*config)

	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	return &DB{sql: db}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.sql.PingContext(ctx)
}

func (d *DB) Ready(ctx context.Context) error {
	var n int
	err := d.sql.QueryRowContext(ctx, "SELECT 1").Scan(&n)
	if err != nil {
		return err
	}
	return nil
}

func (d *DB) Raw() *sql.DB {
	return d.sql
}

func (d *DB) Close() error {
	return d.sql.Close()
}
