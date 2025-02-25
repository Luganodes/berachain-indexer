package events

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/models"
	"bera_indexer/internal/repository"
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/sync/errgroup"
)

type ProcessedEvents struct {
	Deposits      []models.Deposit
	Incentives    []models.Incentive
	BlockRewards  []models.BlockReward
	Distributions []models.Distribution

	depositMu      sync.Mutex
	incentiveMu    sync.Mutex
	blockRewardMu  sync.Mutex
	distributionMu sync.Mutex
}

type EventProcessorInterface interface {
	ProcessEvents(logs []types.Log) (*ProcessedEvents, error)
}

type EventProcessor struct {
	config       *config.Config
	dbRepository *repository.DbRepository
}

func NewEventProcessor(config *config.Config, dbRepository *repository.DbRepository) *EventProcessor {
	return &EventProcessor{
		config:       config,
		dbRepository: dbRepository,
	}
}

func (ep *EventProcessor) ProcessEvents(ctx context.Context, logs []types.Log) (*ProcessedEvents, error) {
	result := &ProcessedEvents{
		Deposits:      make([]models.Deposit, 0),
		Incentives:    make([]models.Incentive, 0),
		BlockRewards:  make([]models.BlockReward, 0),
		Distributions: make([]models.Distribution, 0),

		depositMu:      sync.Mutex{},
		incentiveMu:    sync.Mutex{},
		blockRewardMu:  sync.Mutex{},
		distributionMu: sync.Mutex{},
	}

	g, ctx := errgroup.WithContext(ctx)

	for _, log := range logs {
		logCopy := log
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic occurred while processing event: %v", r)
				}
			}()
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return ep.processEvent(ctx, logCopy, result)
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	log.Printf("Found %d deposits, %d block rewards, %d distributions, %d incentives", len(result.Deposits), len(result.BlockRewards), len(result.Distributions), len(result.Incentives))
	return result, nil
}

func (ep *EventProcessor) processEvent(ctx context.Context, log types.Log, result *ProcessedEvents) error {
	eventName, ok := ep.config.Events[log.Topics[0].Hex()]
	if !ok {
		return fmt.Errorf("event not found: %v", log.Topics[0])
	}

	switch eventName {
	case "Deposit":
		deposit, err := ep.processDeposit(ctx, log)
		if err != nil {
			return fmt.Errorf("failed to process deposit: %w", err)
		}
		if deposit != nil {
			result.depositMu.Lock()
			result.Deposits = append(result.Deposits, *deposit)
			result.depositMu.Unlock()
		}
	case "BlockRewardProcessed":
		reward, err := ep.processBlockReward(ctx, log)
		if err != nil {
			return fmt.Errorf("failed to process block reward: %w", err)
		}
		if reward != nil {
			result.blockRewardMu.Lock()
			result.BlockRewards = append(result.BlockRewards, *reward)
			result.blockRewardMu.Unlock()
		}
	case "Distributed":
		distribution, err := ep.processDistribution(ctx, log)
		if err != nil {
			return fmt.Errorf("failed to process distribution: %w", err)
		}
		if distribution != nil {
			result.distributionMu.Lock()
			result.Distributions = append(result.Distributions, *distribution)
			result.distributionMu.Unlock()
		}
	case "IncentivesProcessed":
		incentive, err := ep.processIncentive(ctx, log)
		if err != nil {
			return fmt.Errorf("failed to process incentive: %w", err)
		}
		if incentive != nil {
			result.incentiveMu.Lock()
			result.Incentives = append(result.Incentives, *incentive)
			result.incentiveMu.Unlock()
		}
	}
	return nil
}
