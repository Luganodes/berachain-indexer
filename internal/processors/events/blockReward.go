package events

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
)

func (p *EventProcessor) processBlockReward(ctx context.Context, log types.Log) (*models.BlockReward, error) {
	validatorId := log.Topics[1].Hex()
	if !utils.IsValidValidator(p.config.Validators, validatorId) {
		return nil, nil
	}

	exists, err := (*p.dbRepository).DoesTransactionExists(ctx, "blockRewards", log.TxHash.Hex(), log.Index)
	if err != nil {
		return nil, fmt.Errorf("failed to check if block reward exists: %w", err)
	}
	if exists {
		return nil, nil
	}

	abi := p.config.Contracts.BlockRewardContract.ABI
	decoded, err := abi.Unpack("BlockRewardProcessed", log.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack log data: %w", err)
	}

	baseRate := decoded[1]
	rewardRate := decoded[2]

	blockReward := models.BlockReward{
		Validator:       validatorId,
		BaseRate:        fmt.Sprintf("%v", baseRate),
		RewardRate:      fmt.Sprintf("%v", rewardRate),
		TransactionHash: log.TxHash.Hex(),
		LogIndex:        log.Index,
		BlockNumber:     log.BlockNumber,
		ToContract:      log.Address.Hex(),
	}
	return &blockReward, nil
}
