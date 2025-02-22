package processors

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/repository"
	"context"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/core/types"
)

type Processor interface {
	ProcessTransactionLogs(ctx context.Context, logs []types.Log, config *config.Config) error
}

type processor struct {
	ethereumRepository *repository.EthereumRepository
	dbRepository       *repository.DbRepository
	config             *config.Config
	eventHandlers      map[string]func(context.Context, types.Log) error
}

func NewProcessor(ethereumRepository *repository.EthereumRepository, dbRepository *repository.DbRepository, config *config.Config) Processor {
	p := &processor{
		ethereumRepository: ethereumRepository,
		dbRepository:       dbRepository,
		config:             config,
	}

	p.eventHandlers = map[string]func(context.Context, types.Log) error{
		"Deposit":              p.processDeposit,
		"BlockRewardProcessed": p.processBlockReward,
		"IncentivesProcessed":  p.processIncentive,
		"Distributed":          p.processDistribution,
	}

	return p
}

func (p *processor) ProcessTransactionLogs(ctx context.Context, logs []types.Log, config *config.Config) error {

	sort.Slice(logs, func(i, j int) bool {
		if logs[i].BlockNumber == logs[j].BlockNumber {
			return logs[i].Index < logs[j].Index
		}
		return logs[i].BlockNumber < logs[j].BlockNumber
	})

	for _, log := range logs {
		if err := p.processLog(ctx, log, config); err != nil {
			return fmt.Errorf("failed to process log: %v: %w", log.TxHash, err)
		}
	}
	return nil
}

func (p *processor) processLog(ctx context.Context, transactionLog types.Log, config *config.Config) error {
	eventName, ok := config.Events[transactionLog.Topics[0].Hex()]
	if !ok {
		return fmt.Errorf("event not found: %v", transactionLog.Topics[0])
	}

	handler, ok := p.eventHandlers[eventName]
	if !ok {
		return fmt.Errorf("no handler for event: %s", eventName)
	}
	return handler(ctx, transactionLog)
}
