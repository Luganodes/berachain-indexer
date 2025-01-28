package repository

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/utils"
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

type EthereumRepository interface {
	GetLatestBlock(ctx context.Context) (uint64, error)
	GetBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error)
	GetTransactionSender(ctx context.Context, txHash string) (string, error)
	CalcTransactionFee(txHash string) (float64, error)
	FetchContractLogs(ctx context.Context, startBlock, endBlock uint64) ([]types.Log, error)
}

type ethereumRepository struct {
	client *ethclient.Client
	config *config.Config
}

func NewEthereumRepository(client *ethclient.Client, config *config.Config) EthereumRepository {
	return &ethereumRepository{
		client: client,
		config: config,
	}
}

func (r *ethereumRepository) GetLatestBlock(ctx context.Context) (uint64, error) {
	block, err := r.client.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch block number: %w", err)
	}
	return block, nil
}

func (r *ethereumRepository) GetBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error) {
	block, err := r.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to fetch block: %w", err)
	}
	return time.Unix(int64(block.Time()), 0), nil
}

func (r *ethereumRepository) GetTransactionSender(ctx context.Context, txHash string) (string, error) {
	tx, isPending, err := r.client.TransactionByHash(ctx, common.HexToHash(txHash))
	if err != nil {
		return "", err
	}
	if isPending {
		return "", fmt.Errorf("transaction is still pending")
	}
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return "", err
	}

	return from.Hex(), nil
}

func (r *ethereumRepository) CalcTransactionFee(txHash string) (float64, error) {
	receipt, err := r.client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch transaction receipt: %w", err)
	}
	gasUsed := receipt.GasUsed
	gasPrice := receipt.EffectiveGasPrice
	transactionFee := new(big.Int).Mul(big.NewInt(int64(gasUsed)), gasPrice)
	return utils.ConvertWeiToEther(transactionFee), nil
}

func (r *ethereumRepository) FetchContractLogs(ctx context.Context, startBlock, endBlock uint64) ([]types.Log, error) {
	var (
		concurrentBatches = r.config.ConcurrentBatches
		batchSize         = r.config.BatchSize
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	totalBlocks := endBlock - startBlock + 1
	bar := progressbar.Default(int64(totalBlocks))

	g, ctx := errgroup.WithContext(ctx)
	semaphore := make(chan struct{}, concurrentBatches)
	logsChan := make(chan []types.Log)

	var logs []types.Log
	var mu sync.Mutex
	go func() {
		for batchLogs := range logsChan {
			mu.Lock()
			logs = append(logs, batchLogs...)
			mu.Unlock()
		}
	}()

	for start := startBlock; start <= endBlock; start += batchSize {
		end := min(start+batchSize, endBlock)
		semaphore <- struct{}{}

		start, end := start, end // Avoid closure capture issues
		g.Go(func() error {
			defer func() { <-semaphore }()
			batchLogs, err := r.fetchLogsBatch(ctx, start, end)
			if err != nil {
				cancel()
				return err
			}

			select {
			case logsChan <- batchLogs:
			case <-ctx.Done():
				return ctx.Err()
			}

			bar.Add(int(end - start))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		close(logsChan)
		return nil, err
	}
	close(logsChan)

	return logs, nil
}

func (r *ethereumRepository) fetchLogsBatch(ctx context.Context, fromBlock, toBlock uint64) ([]types.Log, error) {
	eventHashes := []common.Hash{}
	for eventHash := range r.config.Events {
		eventHashes = append(eventHashes, common.HexToHash(eventHash))
	}

	contractAddresses := []common.Address{
		r.config.Contracts.DistributionContract.Address,
		r.config.Contracts.DepositContract.Address,
		r.config.Contracts.BlockRewardContract.Address,
	}
	contractAddresses = append(contractAddresses, r.config.Contracts.VaultContracts...)

	// todo: need to check if ToBlock is inclusive or exclusive
	// assming exclusive and making an extra request
	query := ethereum.FilterQuery{
		Addresses: contractAddresses,
		Topics:    [][]common.Hash{eventHashes},
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
	}

	maxRetries := r.config.MaxRetries

	for retries := 0; retries < int(maxRetries); retries++ {
		if logs, err := r.client.FilterLogs(ctx, query); err == nil {
			return logs, nil
		}
		time.Sleep(time.Second * time.Duration(retries+1))
	}

	return nil, fmt.Errorf("failed to fetch contract logs after %d retries", maxRetries)
}
