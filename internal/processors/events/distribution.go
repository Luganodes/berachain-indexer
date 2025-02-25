package events

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *EventProcessor) processDistribution(ctx context.Context, log types.Log) (*models.Distribution, error) {
	validatorId := log.Topics[1].Hex()
	if !utils.IsValidValidator(p.config.Validators, validatorId) {
		return nil, nil
	}

	exists, err := (*p.dbRepository).DoesTransactionExists(ctx, "distributions", log.TxHash.Hex(), log.Index)
	if err != nil {
		return nil, fmt.Errorf("failed to check if distribution exists: %w", err)
	}
	if exists {
		return nil, nil
	}

	abi := p.config.Contracts.DistributorContract.ABI
	decoded, err := abi.Unpack("Distributed", log.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack log data: %w", err)
	}

	receiver := common.HexToAddress(log.Topics[3].Hex())
	amount := decoded[0]

	distribution := &models.Distribution{
		Validator:       validatorId,
		Receiver:        receiver.Hex(),
		Amount:          fmt.Sprintf("%v", amount),
		TransactionHash: log.TxHash.Hex(),
		LogIndex:        log.Index,
		BlockNumber:     log.BlockNumber,
		ToContract:      log.Address.Hex(),
	}
	return distribution, nil
}
