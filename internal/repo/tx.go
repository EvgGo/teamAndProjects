package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier позволяет репозиториям одинаково работать внутри транзакции и без нее
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// withTx кладет pgx.Tx в context (внутренне используется TxManager)
func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// querierFromCtx возвращает транзакцию из context, если она есть
func querierFromCtx(ctx context.Context, pool *pgxpool.Pool) Querier {
	if v := ctx.Value(txKey{}); v != nil {
		if tx, ok := v.(pgx.Tx); ok && tx != nil {
			return tx
		}
	}
	return pool
}

// TxManager - простой менеджер транзакций
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithinTx запускает fn в транзакции
// - создает tx
// - кладет tx в ctx
// - коммитит при успехе
// - роллбэчит при ошибке/панике
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		// Если была паника - откатим и пробросим дальше
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		// Если ошибка - rollback
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		// Иначе commit
		if e := tx.Commit(ctx); e != nil {
			err = fmt.Errorf("commit tx: %w", e)
		}
	}()

	txCtx := withTx(ctx, tx)
	err = fn(txCtx)
	return err
}
