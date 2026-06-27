package migration

import (
	"bytes"
	"context"
	"embed"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"xmeco/internal/repository/postgres"
)

//go:embed sql/*.up.sql
var upFiles embed.FS

// Run executes all pending migrations in order.
// It creates the schema_migrations table if it doesn't exist,
// handles existing databases by detecting already-applied tables,
// and runs any pending up-scripts.
func Run(ctx context.Context, pool postgres.DBTX) error {
	// 1. Ensure schema_migrations table
	if _, err := pool.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			title   VARCHAR(200),
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)`); err != nil {
		return err
	}

	// 2. Get list of embedded up migrations
	files, err := listMigrationFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	// 3. If no migrations recorded yet, auto-detect current version
	var maxApplied int
	err = pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxApplied)
	if err != nil {
		return err
	}

	if maxApplied == 0 {
		maxApplied = detectExistingVersion(ctx, pool, files)
		slog.Info("migrate: detected existing database", "version", maxApplied)
	}

	// 4. Run pending migrations
	for _, f := range files {
		if f.version <= maxApplied {
			continue
		}

		slog.Info("migrate: applying", "version", f.version, "title", f.title)
		sql, err := upFiles.ReadFile("sql/" + f.name)
		if err != nil {
			return err
		}
		// Strip UTF-8 BOM if present (some editors add it silently)
		sql = bytes.TrimPrefix(sql, []byte{0xEF, 0xBB, 0xBF})

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return err
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, title) VALUES ($1,$2)`, f.version, f.title); err != nil {
			tx.Rollback(ctx)
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
		slog.Info("migrate: applied", "version", f.version, "title", f.title)
	}

	return nil
}

type migrationFile struct {
	version int
	title   string
	name    string
}

func listMigrationFiles() ([]migrationFile, error) {
	entries, err := upFiles.ReadDir("sql")
	if err != nil {
		return nil, err
	}
	var files []migrationFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		name := e.Name()
		// Parse: 000001_init_schema.up.sql → 1, "init_schema"
		parts := strings.SplitN(name, "_", 3)
		if len(parts) < 2 {
			continue
		}
		ver, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		title := strings.TrimSuffix(parts[len(parts)-1], ".up.sql")
		files = append(files, migrationFile{version: ver, title: title, name: name})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

// detectExistingVersion checks which key tables exist and returns the
// highest migration version that corresponds to those tables.
func detectExistingVersion(ctx context.Context, pool postgres.DBTX, files []migrationFile) int {
	// Check for electricity_price table (migration 011)
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='electricity_price')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 11)
		return 11
	}

	// Check for alarm dedup index (migration 010)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM pg_indexes WHERE indexname='idx_alarm_log_dedup')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 10)
		return 10
	}

	// Check for project_user table (migration 009)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='project_user')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 9)
		return 9
	}

	// Check for scheduled_task table (migration 008)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='scheduled_task')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 8)
		return 8
	}

	// Check for last_online_at column on device (migration 007)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name='device' AND column_name='last_online_at')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 7)
		return 7
	}

	// Check for gateway_imei column type VARCHAR(64) on gateway_config (migration 006)
	var colType string
	err = pool.QueryRow(ctx,
		`SELECT data_type FROM information_schema.columns WHERE table_name='gateway_config' AND column_name='gateway_imei'`).Scan(&colType)
	if err == nil && strings.Contains(strings.ToLower(colType), "varying") {
		markApplied(ctx, pool, files, 6)
		return 6
	}

	// Check for city table (from migration 005)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='city')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 5)
		return 5
	}

	// Check for weather_cache table (from migration 004)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='weather_cache')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 4)
		return 4
	}

	// Check for dashboard_config (from migration 003)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='dashboard_config')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 3)
		return 3
	}

	// Check for role table (from migration 002)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='role')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 2)
		return 2
	}

	// Check for users table (from migration 001)
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='users')`).Scan(&exists)
	if err == nil && exists {
		markApplied(ctx, pool, files, 1)
		return 1
	}

	return 0
}

func markApplied(ctx context.Context, pool postgres.DBTX, files []migrationFile, maxVersion int) {
	for _, f := range files {
		if f.version > maxVersion {
			break
		}
		_, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (version, title) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			f.version, f.title)
		if err != nil {
			slog.Warn("migrate: mark applied failed", "version", f.version, "err", err)
		}
	}
}
