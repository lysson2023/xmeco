package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is a database handle interface satisfied by both *pgxpool.Pool (production)
// and pgxmock.PgxPoolIface (testing). It exposes only the methods used by repositories.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Ping(ctx context.Context) error
	Close()
}

// Compile-time check: *pgxpool.Pool satisfies DBTX.
var _ DBTX = (*pgxpool.Pool)(nil)
