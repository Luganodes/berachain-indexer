package events

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *EventProcessor) processIncentive(ctx context.Context, log types.Log) (*models.Incentive, error) {
	validatorId := log.Topics[1].Hex()
	if !utils.IsValidValidator(p.config.Validators, validatorId) {
		return nil, nil
	}

	exists, err := (*p.dbRepository).DoesTransactionExists(ctx, "incentives", log.TxHash.Hex(), log.Index)
	if err != nil {
		return nil, fmt.Errorf("failed to check if incentive exists: %w", err)
	}
	if exists {
		return nil, nil
	}

	abi := p.config.Contracts.IncentiveAbi
	decoded, err := abi.Unpack("IncentivesProcessed", log.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack log data: %w", err)
	}

	token := common.HexToAddress(log.Topics[2].Hex())
	bgtEmitted := decoded[0]
	amount := decoded[1]

	incentive := models.Incentive{
		Validator:       validatorId,
		Token:           token.Hex(),
		BGTEmitted:      fmt.Sprintf("%v", bgtEmitted),
		Amount:          fmt.Sprintf("%v", amount),
		TransactionHash: log.TxHash.Hex(),
		LogIndex:        log.Index,
		BlockNumber:     log.BlockNumber,
		ToContract:      log.Address.Hex(),
	}
	return &incentive, nil
}
