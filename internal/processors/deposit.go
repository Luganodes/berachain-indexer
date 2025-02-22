package processors

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *processor) processDeposit(ctx context.Context, log types.Log) error {
	transaction, err := p.FormTransaction(ctx, log, "deposits")
	if err != nil {
		return fmt.Errorf("failed to form transaction: %w", err)
	}
	if transaction == nil {
		return nil
	}

	abi := p.config.Contracts.DepositContract.ABI
	decoded, err := abi.Unpack("Deposit", log.Data)
	if err != nil {
		return fmt.Errorf("failed to unpack log data: %w", err)
	}

	pubkey := "0x" + common.Bytes2Hex(decoded[0].([]byte))
	if !utils.IsValidValidator(p.config.Validators, pubkey) {
		return nil
	}

	credentials := "0x" + common.Bytes2Hex(decoded[1].([]byte))
	amount := decoded[2]
	signature := "0x" + common.Bytes2Hex(decoded[3].([]byte))

	deposit := models.Deposit{
		Validator:             pubkey,
		Amount:                fmt.Sprintf("%v", amount),
		WithdrawalCredentials: credentials,
		Signature:             signature,
		TransactionHash:       transaction.TransactionHash,
		LogIndex:              transaction.LogIndex,
		BlockNumber:           transaction.BlockNumber,
		BlockTimestamp:        transaction.BlockTimestamp,
		Fee:                   transaction.Fee,
		TransactionFrom:       transaction.TransactionFrom,
		ToContract:            transaction.ToContract,
	}

	err = (*p.dbRepository).AddDeposit(ctx, deposit)
	if err != nil {
		return fmt.Errorf("failed to add deposit: %w", err)
	}
	fmt.Println("Deposit found from: ", deposit.TransactionFrom)

	err = (*p.dbRepository).UpsertDelegator(ctx, deposit.TransactionFrom, deposit.Validator, deposit.Amount)
	if err != nil {
		return fmt.Errorf("failed to upsert delegator: %w", err)
	}
	return nil
}
