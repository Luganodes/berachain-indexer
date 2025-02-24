package processors

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/processors/events"
	"bera_indexer/internal/repository"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type TransactionProcessor interface {
	FetchLogs(ctx context.Context, startBlock, endBlock uint64) ([]types.Log, error)
	GetTransactionsData(ctx context.Context, txHashes []string) (map[string]*TransactionData, error)
}

type transactionProcessor struct {
	config             *config.Config
	ethereumRepository *repository.EthereumRepository
}

func NewtransactionProcessor(config *config.Config, ethereumRepository repository.EthereumRepository) *transactionProcessor {
	return &transactionProcessor{
		config:             config,
		ethereumRepository: &ethereumRepository,
	}
}

func (tp *transactionProcessor) FetchLogs(ctx context.Context, startBlock, endBlock uint64) ([]types.Log, error) {
	eventHashes := []common.Hash{}
	for eventHash := range tp.config.Events {
		eventHashes = append(eventHashes, common.HexToHash(eventHash))
	}

	contractAddresses := []common.Address{
		tp.config.Contracts.DistributionContract.Address,
		tp.config.Contracts.DepositContract.Address,
		tp.config.Contracts.BlockRewardContract.Address,
	}
	contractAddresses = append(contractAddresses, tp.config.Contracts.VaultContracts...)

	logs, err := (*tp.ethereumRepository).FetchContractLogs(ctx, startBlock, endBlock, contractAddresses, [][]common.Hash{eventHashes})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contract logs: %w", err)
	}

	return logs, nil
}

func (tp *transactionProcessor) GetTxHashesAndBlockNumbers(ctx context.Context, processedEvents *events.ProcessedEvents) ([]string, []uint64, error) {
	txHashMap := make(map[string]struct{})
	blockNumberMap := make(map[uint64]struct{})

	addToMaps := func(txHash string, blockNum uint64) {
		txHashMap[txHash] = struct{}{}
		blockNumberMap[blockNum] = struct{}{}
	}

	for _, event := range processedEvents.Deposits {
		addToMaps(event.TransactionHash, event.BlockNumber)
	}
	for _, event := range processedEvents.Incentives {
		addToMaps(event.TransactionHash, event.BlockNumber)
	}
	for _, event := range processedEvents.BlockRewards {
		addToMaps(event.TransactionHash, event.BlockNumber)
	}
	for _, event := range processedEvents.Distributions {
		addToMaps(event.TransactionHash, event.BlockNumber)
	}

	// Convert maps to slices
	txHashes := make([]string, 0, len(txHashMap))
	for txHash := range txHashMap {
		txHashes = append(txHashes, txHash)
	}

	blockNumbers := make([]uint64, 0, len(blockNumberMap))
	for blockNum := range blockNumberMap {
		blockNumbers = append(blockNumbers, blockNum)
	}

	return txHashes, blockNumbers, nil
}

type TransactionData struct {
	Sender string
	Fee    float64
}

func (tp *transactionProcessor) GetTransactionsData(ctx context.Context, txHashes []string) (map[string]*TransactionData, error) {
	txData := make(map[string]*TransactionData)

	log.Printf("Fetching transaction senders for %d transactions", len(txHashes))
	txSenders, err := (*tp.ethereumRepository).GetTransactionSenders(ctx, txHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch transaction senders: %w", err)
	}
	log.Printf("Fetched transaction senders for %d transactions", len(txSenders))

	log.Printf("Calculating transaction fee for %d transactions", len(txHashes))
	txFees, err := (*tp.ethereumRepository).CalcTransactionFeeBatch(ctx, txHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch transaction fee: %w", err)
	}
	log.Printf("Calculated transaction fee for %d transactions", len(txFees))
	for _, txHash := range txHashes {
		txData[txHash] = &TransactionData{
			Sender: txSenders[txHash],
			Fee:    txFees[txHash],
		}
	}
	return txData, nil
}

func (tp *transactionProcessor) FillTransactionData(ctx context.Context, events *events.ProcessedEvents, txData map[string]*TransactionData, blockTimestamps map[uint64]time.Time) {
	for i := range events.Deposits {
		events.Deposits[i].TransactionFrom = txData[events.Deposits[i].TransactionHash].Sender
		events.Deposits[i].Fee = txData[events.Deposits[i].TransactionHash].Fee
		events.Deposits[i].BlockTimestamp = blockTimestamps[events.Deposits[i].BlockNumber]
	}
	for i := range events.BlockRewards {
		events.BlockRewards[i].TransactionFrom = txData[events.BlockRewards[i].TransactionHash].Sender
		events.BlockRewards[i].Fee = txData[events.BlockRewards[i].TransactionHash].Fee
		events.BlockRewards[i].BlockTimestamp = blockTimestamps[events.BlockRewards[i].BlockNumber]
	}
	for i := range events.Distributions {
		events.Distributions[i].TransactionFrom = txData[events.Distributions[i].TransactionHash].Sender
		events.Distributions[i].Fee = txData[events.Distributions[i].TransactionHash].Fee
		events.Distributions[i].BlockTimestamp = blockTimestamps[events.Distributions[i].BlockNumber]
	}
	for i := range events.Incentives {
		events.Incentives[i].TransactionFrom = txData[events.Incentives[i].TransactionHash].Sender
		events.Incentives[i].Fee = txData[events.Incentives[i].TransactionHash].Fee
		events.Incentives[i].BlockTimestamp = blockTimestamps[events.Incentives[i].BlockNumber]
	}
}
