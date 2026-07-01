package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is a database handle interface satisfied by both *pgxpool.Pool (production)
// and pgxmock.PgxPoolIface (testing). It exposes only the methods used by repositories
// and services. pgx.Tx also satisfies this interface, enabling alarm and other service
// code to participate in caller-managed transactions.
// Ping and Close are intentionally excluded — connection-pool lifecycle management is the
// caller's responsibility and should use the concrete *pgxpool.Pool type.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// Compile-time check: *pgxpool.Pool satisfies DBTX.
var _ DBTX = (*pgxpool.Pool)(nil)
