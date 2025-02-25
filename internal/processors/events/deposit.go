package events

import (
	"bera_indexer/internal/models"
	"bera_indexer/internal/utils"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (p *EventProcessor) processDeposit(ctx context.Context, log types.Log) (*models.Deposit, error) {
	abi := p.config.Contracts.DepositContract.ABI
	decoded, err := abi.Unpack("Deposit", log.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack log data: %w", err)
	}

	pubkey := "0x" + common.Bytes2Hex(decoded[0].([]byte))
	if !utils.IsValidValidator(p.config.Validators, pubkey) {
		return nil, nil
	}

	exists, err := (*p.dbRepository).DoesTransactionExists(ctx, "deposits", log.TxHash.Hex(), log.Index)
	if err != nil {
		return nil, fmt.Errorf("failed to check if deposit exists: %w", err)
	}
	if exists {
		return nil, nil
	}

	credentials := "0x" + common.Bytes2Hex(decoded[1].([]byte))
	amount := decoded[2]
	signature := "0x" + common.Bytes2Hex(decoded[3].([]byte))

	deposit := &models.Deposit{
		Validator:             pubkey,
		Amount:                fmt.Sprintf("%v", amount),
		WithdrawalCredentials: credentials,
		Signature:             signature,
		TransactionHash:       log.TxHash.Hex(),
		LogIndex:              log.Index,
		BlockNumber:           log.BlockNumber,
		ToContract:            log.Address.Hex(),
	}
	return deposit, nil
}
