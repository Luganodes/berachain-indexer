package processors

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *processor) processDistribution(ctx context.Context, log types.Log) error {
	validatorId := log.Topics[1].Hex()
	if !utils.IsValidValidator(p.config.Validators, validatorId) {
		return nil
	}

	transaction, err := p.FormTransaction(ctx, log, "distributions")
	if err != nil {
		return fmt.Errorf("failed to form transaction: %w", err)
	}
	if transaction == nil {
		return nil
	}

	abi := p.config.Contracts.DistributionContract.ABI
	decoded, err := abi.Unpack("Distributed", log.Data)
	if err != nil {
		return fmt.Errorf("failed to unpack log data: %w", err)
	}

	receiver := common.HexToAddress(log.Topics[3].Hex())
	amount := decoded[0]

	distribution := models.Distribution{
		Validator:       validatorId,
		Receiver:        receiver.Hex(),
		Amount:          fmt.Sprintf("%v", amount),
		TransactionHash: transaction.TransactionHash,
		LogIndex:        transaction.LogIndex,
		BlockNumber:     transaction.BlockNumber,
		BlockTimestamp:  transaction.BlockTimestamp,
		Fee:             transaction.Fee,
		TransactionFrom: transaction.TransactionFrom,
		ToContract:      transaction.ToContract,
	}

	err = (*p.dbRepository).AddDistribution(ctx, distribution)
	if err != nil {
		return fmt.Errorf("failed to add distribution: %w", err)
	}
	fmt.Println("Distribution processed for block number: ", distribution.BlockNumber)
	return nil
}
