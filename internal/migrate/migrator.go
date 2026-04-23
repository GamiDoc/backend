package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gamidoc/backend/internal/storage/postgres"
)

type StatusEntry struct {
	Name    string
	Applied bool
}

type Migrator struct {
	db            *postgres.DB
	migrationsDir string
}

func NewMigrator(db *postgres.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
	}
}

func (m *Migrator) Status(ctx context.Context) ([]StatusEntry, error) {
	if err := m.ensureMetaTable(ctx); err != nil {
		return nil, err
	}

	files, err := m.migrationFiles()
	if err != nil {
		return nil, err
	}

	applied, err := m.appliedSet(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]StatusEntry, 0, len(files))
	for _, file := range files {
		result = append(result, StatusEntry{
			Name:    file,
			Applied: applied[m.key(file)],
		})
	}

	return result, nil
}

func (m *Migrator) Up(ctx context.Context) ([]StatusEntry, error) {
	if err := m.ensureMetaTable(ctx); err != nil {
		return nil, err
	}

	files, err := m.migrationFiles()
	if err != nil {
		return nil, err
	}

	applied, err := m.appliedSet(ctx)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		key := m.key(file)
		if applied[key] {
			continue
		}

		content, err := os.ReadFile(filepath.Join(m.migrationsDir, file))
		if err != nil {
			return nil, err
		}

		tx, err := m.db.Raw().BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("apply migration %s: %w", file, err)
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO app_meta (key, value) VALUES ($1, $2)`,
			key,
			time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	return m.Status(ctx)
}

func (m *Migrator) ensureMetaTable(ctx context.Context) error {
	_, err := m.db.Raw().ExecContext(
		ctx,
		`
		CREATE TABLE IF NOT EXISTS app_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		`,
	)
	return err
}

func (m *Migrator) appliedSet(ctx context.Context) (map[string]bool, error) {
	rows, err := m.db.Raw().QueryContext(
		ctx,
		`SELECT key FROM app_meta WHERE key LIKE 'migration:%'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		result[key] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (m *Migrator) migrationFiles() ([]string, error) {
	entries, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			files = append(files, name)
		}
	}

	sort.Strings(files)
	return files, nil
}

func (m *Migrator) key(name string) string {
	return "migration:" + name
}

var _ = sql.ErrNoRows
