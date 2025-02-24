package processors

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/processors/events"
	"bera_indexer/internal/repository"
	"context"
	"fmt"
	"log"
)

type Processor struct {
	eventProcessor *events.EventProcessor
	txProcessor    *transactionProcessor
	dbRepository   *repository.DbRepository
}

func NewProcessor(
	ethereumRepo *repository.EthereumRepository,
	dbRepo *repository.DbRepository,
	config *config.Config,
) *Processor {
	return &Processor{
		eventProcessor: events.NewEventProcessor(config),
		txProcessor:    NewtransactionProcessor(config, *ethereumRepo),
		dbRepository:   dbRepo,
	}
}

func (p *Processor) ProcessTransactionLogs(ctx context.Context, startBlock, endBlock uint64) error {
	log.Printf("Fetching logs from block number %d to %d", startBlock, endBlock)
	logs, err := p.txProcessor.FetchLogs(ctx, startBlock, endBlock)
	if err != nil {
		return err
	}
	log.Printf("Fetched %d logs from block number %d to %d", len(logs), startBlock, endBlock)

	events, err := p.eventProcessor.ProcessEvents(logs)
	if err != nil {
		return err
	}

	txHashes, blockNumbers, err := p.txProcessor.GetTxHashesAndBlockNumbers(ctx, events)
	if err != nil {
		return err
	}
	txData, err := p.txProcessor.GetTransactionsData(ctx, txHashes)
	if err != nil {
		return err
	}
	log.Printf("Fetching block timestamps for %d blocks", len(blockNumbers))
	blockTimestamps, err := (*p.txProcessor.ethereumRepository).GetBlockTimestamps(ctx, blockNumbers)
	if err != nil {
		return err
	}
	log.Printf("Fetched block timestamps for %d blocks", len(blockTimestamps))
	p.txProcessor.FillTransactionData(ctx, events, txData, blockTimestamps)
	if err := p.saveToDatabase(ctx, events); err != nil {
		return err
	}
	return nil
}

func (p *Processor) saveToDatabase(ctx context.Context, events *events.ProcessedEvents) error {
	if err := (*p.dbRepository).AddDeposits(ctx, events.Deposits); err != nil {
		return fmt.Errorf("failed to bulk add deposits: %w", err)
	}
	if err := (*p.dbRepository).AddBlockRewards(ctx, events.BlockRewards); err != nil {
		return fmt.Errorf("failed to bulk add block rewards: %w", err)
	}
	if err := (*p.dbRepository).AddDistributions(ctx, events.Distributions); err != nil {
		return fmt.Errorf("failed to bulk add distributions: %w", err)
	}
	if err := (*p.dbRepository).AddIncentives(ctx, events.Incentives); err != nil {
		return fmt.Errorf("failed to bulk add incentives: %w", err)
	}
	return nil
}
