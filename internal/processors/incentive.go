package processors

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *processor) processIncentive(ctx context.Context, log types.Log) error {
	validatorId := log.Topics[1].Hex()
	if !utils.IsValidValidator(p.config.Validators, validatorId) {
		return nil
	}

	transaction, err := p.FormTransaction(ctx, log, "incentives")
	if err != nil {
		return fmt.Errorf("failed to form transaction: %w", err)
	}
	if transaction == nil {
		return nil
	}

	abi := p.config.Contracts.VaultContractAbi
	decoded, err := abi.Unpack("IncentivesProcessed", log.Data)
	if err != nil {
		return fmt.Errorf("failed to unpack log data: %w", err)
	}

	token := common.HexToAddress(log.Topics[2].Hex())
	bgtEmitted := decoded[0]
	amount := decoded[1]

	incentive := models.Incentive{
		Validator:       validatorId,
		Token:           token.Hex(),
		BGTEmitted:      fmt.Sprintf("%v", bgtEmitted),
		Amount:          fmt.Sprintf("%v", amount),
		TransactionHash: transaction.TransactionHash,
		LogIndex:        transaction.LogIndex,
		BlockNumber:     transaction.BlockNumber,
		BlockTimestamp:  transaction.BlockTimestamp,
		Fee:             transaction.Fee,
		TransactionFrom: transaction.TransactionFrom,
		ToContract:      transaction.ToContract,
	}

	err = (*p.dbRepository).AddIncentive(ctx, incentive)
	if err != nil {
		return fmt.Errorf("failed to add incentive: %w", err)
	}
	fmt.Println("Incentive processed for block number: ", incentive.BlockNumber)
	return nil
}
