package services

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/processors"
	"bera_indexer/internal/repository"
	"context"
	"fmt"
	"log"
)

func SyncTransactions(dbRepository *repository.DbRepository, ethereumRepository *repository.EthereumRepository, config *config.Config) error {
	ctx := context.Background()
	log.Println("Syncing transactions...")

	blockRepository := repository.NewBlockRepository(dbRepository)
	processor := processors.NewProcessor(ethereumRepository, dbRepository, config)
	startBlock, err := blockRepository.FindLastBlockProcessed(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to find last processed block: %w", err)
	}

	latestBlock, err := (*ethereumRepository).GetLatestBlock(ctx)
	if err != nil {
		return err
	}
	if latestBlock <= startBlock {
		log.Printf("Skipping sync (latest block < start block)")
		return nil
	}

	log.Printf("Fetching logs from block %d to %d", startBlock, latestBlock)

	transactionLogs, err := (*ethereumRepository).FetchContractLogs(ctx, startBlock, latestBlock)
	if err != nil {
		return fmt.Errorf("failed to fetch contract logs: %w", err)
	}

	if err := processor.ProcessTransactionLogs(ctx, transactionLogs, config); err != nil {
		return err
	}

	if err := blockRepository.UpdateLastBlockProcessed(ctx, latestBlock); err != nil {
		return fmt.Errorf("failed to update last processed block: %w", err)
	}

	log.Printf("Processed %d logs", len(transactionLogs))
	return nil
}
