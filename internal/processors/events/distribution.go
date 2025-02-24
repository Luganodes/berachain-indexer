package events

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *EventProcessor) processDistribution(log types.Log) (*models.Distribution, error) {
	validatorId := log.Topics[1].Hex()
	if !utils.IsValidValidator(p.config.Validators, validatorId) {
		return nil, nil
	}

	abi := p.config.Contracts.DistributionContract.ABI
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
