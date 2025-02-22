package repository

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/utils"
	"context"
	"fmt"
	"math/big"
	"math/rand"
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

	bar := progressbar.Default(int64(endBlock - startBlock + 1))
	defer bar.Finish()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrentBatches)

	var logs []types.Log
	var logsMu sync.Mutex
	var barMu sync.Mutex

	for start := startBlock; start <= endBlock; start += batchSize {
		end := min(start+batchSize-1, endBlock)

		start, end := start, end // Prevent closure capture issue
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic occurred while fetching logs: %v", r)
				}
			}()

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				err := r.fetchLogsBatch(ctx, start, end, &logs, &logsMu)
				if err != nil {
					return err
				}
				barMu.Lock()
				bar.Add(int(end - start + 1))
				barMu.Unlock()
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *ethereumRepository) fetchLogsBatch(ctx context.Context, fromBlock, toBlock uint64, logs *[]types.Log, mu *sync.Mutex) error {
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

	query := ethereum.FilterQuery{
		Addresses: contractAddresses,
		Topics:    [][]common.Hash{eventHashes},
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
	}

	maxRetries := r.config.MaxRetries

	for retries := 0; retries < int(maxRetries); retries++ {
		if response, err := r.client.FilterLogs(ctx, query); err == nil {
			mu.Lock()
			*logs = append(*logs, response...)
			mu.Unlock()
			return nil
		}

		jitter := time.Duration(rand.Int63n(int64(time.Second)))
		backoff := time.Second*time.Duration(1<<retries) + jitter
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("failed to fetch logs from %d to %d after %d retries", fromBlock, toBlock, maxRetries)
}
