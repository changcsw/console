package postgres

import (
	"context"

	"github.com/csw/console/services/admin-api/internal/app/command"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SyncStore struct {
	pool *pgxpool.Pool
}

func NewSyncStore(pool *pgxpool.Pool) *SyncStore { return &SyncStore{pool: pool} }

func syncRepoFrom(db DBTX) command.SectionSyncRepository {
	return &SyncRepo{db: db}
}

func (s *SyncStore) Repository() command.SectionSyncRepository {
	return syncRepoFrom(s.pool)
}

func (s *SyncStore) InTx(ctx context.Context, fn func(repo command.SectionSyncRepository) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(syncRepoFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
