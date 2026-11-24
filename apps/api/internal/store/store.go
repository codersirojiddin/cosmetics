// Package store contains every SQL query. Methods live on Queries so they can run
// either on the pool or inside a transaction (see Store.InTx).
package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("store: not found")

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Queries struct{ db DBTX }

type Store struct {
	pool *pgxpool.Pool
	*Queries
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool, Queries: &Queries{db: pool}} }

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// InTx runs fn in a transaction; it rolls back when fn returns an error.
func (s *Store) InTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&Queries{db: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func list[T any](rows pgx.Rows, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
}

func one[T any](rows pgx.Rows, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	v, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// pgErr returns the Postgres error code and constraint name, if err is one.
func pgErr(err error) (code, constraint string) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code, pe.ConstraintName
	}
	return "", ""
}

// IsUnique reports a unique violation, optionally for a constraint containing `name`.
func IsUnique(err error, name string) bool {
	code, c := pgErr(err)
	return code == "23505" && (name == "" || strings.Contains(c, name))
}

func IsForeignKey(err error) bool { code, _ := pgErr(err); return code == "23503" }
func IsCheck(err error) bool      { code, _ := pgErr(err); return code == "23514" }
func IsInvalidText(err error) bool {
	code, _ := pgErr(err)
	return code == "22P02" // invalid input syntax (e.g. malformed uuid)
}

// likePattern builds a safe %q% pattern.
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

// nameExpr is a SQL expression for a user's display name (” when the joined row is missing).
func nameExpr(alias string) string {
	return "COALESCE(NULLIF(btrim(" + alias + ".first_name || ' ' || " + alias + ".last_name), ''), NULLIF(" + alias + ".username, ''), '')"
}
