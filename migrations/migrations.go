package migrations

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/tern/migrate"
)

const (
	versionTable   = "__versions"
	migrationsPath = "migrations"
)

func Migrate(ctx context.Context, db *pgx.Conn) error {
	m, err := migrate.NewMigrator(ctx, db, versionTable)
	if err != nil {
		return err
	}
	if err := m.LoadMigrations(migrationsPath); err != nil {
		return err
	}
	if len(m.Migrations) == 0 {
		return nil
	}
	return m.Migrate(ctx)
}
