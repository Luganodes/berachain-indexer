package repository

import (
	"bera_indexer/internal/config"
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

type BlockRepository interface {
	FindLastBlockProcessed(ctx context.Context, config *config.Config) (uint64, error)
	UpdateLastBlockProcessed(ctx context.Context, blockNumber uint64) error
}

type blockRepository struct {
	dbRepository *DbRepository
	collection   string
}

func NewBlockRepository(dbRepository *DbRepository) BlockRepository {
	return &blockRepository{
		dbRepository: dbRepository,
		collection:   "metadata",
	}
}

func (r *blockRepository) FindLastBlockProcessed(ctx context.Context, config *config.Config) (uint64, error) {
	lastBlockProcessed, err := (*r.dbRepository).FindLastBlockProcessed(ctx)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return config.StartBlock, nil
		}
		return 0, err
	}
	return lastBlockProcessed + 1, nil
}

func (r *blockRepository) UpdateLastBlockProcessed(ctx context.Context, blockNumber uint64) error {
	if err := (*r.dbRepository).InsertBlock(ctx, blockNumber); err != nil {
		return fmt.Errorf("failed to update last processed block: %w", err)
	}
	return nil
}
