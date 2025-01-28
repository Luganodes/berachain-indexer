package processors

import (
	"bera_indexer/internal/models"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
)

func (p *processor) processBlockReward(ctx context.Context, log types.Log) error {
	validatorId := log.Topics[1].Hex()
	if validatorId != p.config.ValidatorId {
		return nil
	}

	transaction, err := p.FormTransaction(ctx, log, "blockRewards")
	if err != nil {
		return fmt.Errorf("failed to form transaction: %w", err)
	}
	if transaction == nil {
		return nil
	}

	abi := p.config.Contracts.BlockRewardContract.ABI
	decoded, err := abi.Unpack("BlockRewardProcessed", log.Data)
	if err != nil {
		return fmt.Errorf("failed to unpack log data: %w", err)
	}

	baseRate := decoded[1]
	rewardRate := decoded[2]

	blockReward := models.BlockReward{
		Validator:       p.config.ValidatorId,
		BaseRate:        fmt.Sprintf("%v", baseRate),
		RewardRate:      fmt.Sprintf("%v", rewardRate),
		TransactionHash: transaction.TransactionHash,
		LogIndex:        transaction.LogIndex,
		BlockNumber:     transaction.BlockNumber,
		BlockTimestamp:  transaction.BlockTimestamp,
		Fee:             transaction.Fee,
		TransactionFrom: transaction.TransactionFrom,
		ToContract:      transaction.ToContract,
	}

	err = (*p.dbRepository).AddBlockReward(ctx, blockReward)
	if err != nil {
		return fmt.Errorf("failed to add block reward: %w", err)
	}
	fmt.Println("Block reward processed for block number: ", blockReward.BlockNumber)
	return nil
}
