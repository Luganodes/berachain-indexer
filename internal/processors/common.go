package processors

import (
	"context"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

type Transaction struct {
	TransactionHash string    `bson:"transactionHash"`
	LogIndex        uint      `bson:"logIndex"`
	BlockNumber     uint64    `bson:"blockNumber"`
	BlockTimestamp  time.Time `bson:"blockTimestamp"`
	Fee             float64   `bson:"fee"`
	TransactionFrom string    `bson:"transactionFrom"`
	ToContract      string    `bson:"toContract"`
}

func (p *processor) FormTransaction(ctx context.Context, transactionLog types.Log, collectionName string) (*Transaction, error) {

	txHash := transactionLog.TxHash.Hex()
	exists, err := (*p.dbRepository).DoesTransactionExist(ctx, collectionName, txHash, transactionLog.Index)
	if err != nil {
		return nil, err
	} else if exists {
		log.Printf("%s transaction already exists: %s", collectionName, txHash)
		return nil, nil
	}

	transactionFrom, err := (*p.ethereumRepository).GetTransactionSender(ctx, txHash)
	if err != nil {
		return nil, err
	}
	transactionFee, err := (*p.ethereumRepository).CalcTransactionFee(txHash)
	if err != nil {
		return nil, err
	}
	blockTimestamp, err := (*p.ethereumRepository).GetBlockTimestamp(ctx, transactionLog.BlockNumber)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		TransactionHash: txHash,
		LogIndex:        transactionLog.Index,
		BlockNumber:     transactionLog.BlockNumber,
		BlockTimestamp:  blockTimestamp,
		Fee:             transactionFee,
		TransactionFrom: transactionFrom,
		ToContract:      transactionLog.Address.Hex(),
	}, nil
}
