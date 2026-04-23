package postgres

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	sql *sql.DB
}

func New(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

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
