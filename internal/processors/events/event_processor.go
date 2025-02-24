package events

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/models"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/core/types"
)

type ProcessedEvents struct {
	Deposits      []models.Deposit
	Incentives    []models.Incentive
	BlockRewards  []models.BlockReward
	Distributions []models.Distribution
}

type EventProcessorInterface interface {
	ProcessEvents(logs []types.Log) (*ProcessedEvents, error)
}

type EventProcessor struct {
	config *config.Config
}

func NewEventProcessor(config *config.Config) *EventProcessor {
	return &EventProcessor{
		config: config,
	}
}

func (ep *EventProcessor) ProcessEvents(logs []types.Log) (*ProcessedEvents, error) {
	result := &ProcessedEvents{
		Deposits:      make([]models.Deposit, 0),
		Incentives:    make([]models.Incentive, 0),
		BlockRewards:  make([]models.BlockReward, 0),
		Distributions: make([]models.Distribution, 0),
	}

	for _, log := range logs {
		if err := ep.processEvent(log, result); err != nil {
			return nil, err
		}
	}
	log.Printf("Found %d deposits, %d block rewards, %d distributions, %d incentives", len(result.Deposits), len(result.BlockRewards), len(result.Distributions), len(result.Incentives))
	return result, nil
}

func (ep *EventProcessor) processEvent(log types.Log, result *ProcessedEvents) error {
	eventName, ok := ep.config.Events[log.Topics[0].Hex()]
	if !ok {
		return fmt.Errorf("event not found: %v", log.Topics[0])
	}

	switch eventName {
	case "Deposit":
		deposit, err := ep.processDeposit(log)
		if err != nil {
			return fmt.Errorf("failed to process deposit: %w", err)
		}
		if deposit != nil {
			result.Deposits = append(result.Deposits, *deposit)
		}
	case "BlockRewardProcessed":
		reward, err := ep.processBlockReward(log)
		if err != nil {
			return fmt.Errorf("failed to process block reward: %w", err)
		}
		if reward != nil {
			result.BlockRewards = append(result.BlockRewards, *reward)
		}
	case "Distributed":
		distribution, err := ep.processDistribution(log)
		if err != nil {
			return fmt.Errorf("failed to process distribution: %w", err)
		}
		if distribution != nil {
			result.Distributions = append(result.Distributions, *distribution)
		}
	case "IncentivesProcessed":
		incentive, err := ep.processIncentive(log)
		if err != nil {
			return fmt.Errorf("failed to process incentive: %w", err)
		}
		if incentive != nil {
			result.Incentives = append(result.Incentives, *incentive)
		}
	}
	return nil
}
