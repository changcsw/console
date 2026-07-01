package postgres

import (
	"context"

	snapshotapp "github.com/csw/console/services/admin-api/internal/app/snapshot"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SnapshotStore struct {
	pool *pgxpool.Pool
}

func NewSnapshotStore(pool *pgxpool.Pool) *SnapshotStore { return &SnapshotStore{pool: pool} }

func snapshotRepoFrom(db DBTX) snapshotapp.Repository {
	return &SnapshotRepo{db: db}
}

func (s *SnapshotStore) Repository() snapshotapp.Repository {
	return snapshotRepoFrom(s.pool)
}

func (s *SnapshotStore) InTx(ctx context.Context, fn func(snapshotapp.Repository) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(snapshotRepoFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
