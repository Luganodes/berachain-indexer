package repository

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/utils"
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
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
	CalcTransactionFee(ctx context.Context, txHash string) (float64, error)
	FetchContractLogs(ctx context.Context, startBlock, endBlock uint64, contracts []common.Address, topics [][]common.Hash) ([]types.Log, error)
	GetBlockTimestamps(ctx context.Context, blockNumbers []uint64) (map[uint64]time.Time, error)
	GetTransactionSenders(ctx context.Context, txHashes []string) (map[string]string, error)
	CalcTransactionFeeBatch(ctx context.Context, txHashes []string) (map[string]float64, error)
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
	operation := func() (uint64, error) {
		block, err := r.client.BlockNumber(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch block number: %w", err)
		}
		return block, nil
	}
	return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

func (r *ethereumRepository) GetBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error) {
	operation := func() (time.Time, error) {
		block, err := r.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to fetch block: %w", err)
		}
		return time.Unix(int64(block.Time()), 0), nil
	}
	return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}
func (r *ethereumRepository) GetTransactionSender(ctx context.Context, txHash string) (string, error) {
	operation := func() (string, error) {
		tx, isPending, err := r.client.TransactionByHash(ctx, common.HexToHash(txHash))
		if err != nil {
			return "", fmt.Errorf("failed to fetch transaction: %w", err)
		}
		if isPending {
			return "", fmt.Errorf("transaction is still pending")
		}
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			return "", fmt.Errorf("failed to get sender: %w", err)
		}
		return from.Hex(), nil
	}
	return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

func (r *ethereumRepository) CalcTransactionFee(ctx context.Context, txHash string) (float64, error) {
	operation := func() (float64, error) {
		receipt, err := r.client.TransactionReceipt(ctx, common.HexToHash(txHash))
		if err != nil {
			return 0, fmt.Errorf("failed to fetch transaction receipt: %w", err)
		}
		gasUsed := receipt.GasUsed
		gasPrice := receipt.EffectiveGasPrice
		transactionFee := new(big.Int).Mul(big.NewInt(int64(gasUsed)), gasPrice)
		return utils.ConvertWeiToEther(transactionFee), nil
	}
	return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

func (r *ethereumRepository) FetchContractLogs(ctx context.Context, startBlock, endBlock uint64, contracts []common.Address, topics [][]common.Hash) ([]types.Log, error) {
	var (
		batchSize = r.config.GetLogsBatchSize
	)

	bar := progressbar.Default(int64(endBlock - startBlock + 1))
	defer bar.Finish()

	g, ctx := errgroup.WithContext(ctx)

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
				err := r.fetchLogsBatch(ctx, start, end, contracts, topics, &logs, &logsMu)
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

func (r *ethereumRepository) fetchLogsBatch(ctx context.Context, fromBlock, toBlock uint64, contracts []common.Address, topics [][]common.Hash, logs *[]types.Log, mu *sync.Mutex) error {
	query := ethereum.FilterQuery{
		Addresses: contracts,
		Topics:    topics,
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
	}

	operation := func() ([]types.Log, error) {
		return r.client.FilterLogs(ctx, query)
	}

	response, err := backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		return fmt.Errorf("failed to fetch logs from block number %d to %d", fromBlock, toBlock)
	}
	mu.Lock()
	*logs = append(*logs, response...)
	mu.Unlock()
	return nil
}

// Generic type for batch processing results
type batchProcessor[K comparable, V any] struct {
	ctx         context.Context
	items       []K
	process     func(K) (V, error)
	results     map[K]V
	mu          sync.Mutex
	progressBar *progressbar.ProgressBar
	barMu       sync.Mutex
}

// Process items in parallel with progress tracking
func (bp *batchProcessor[K, V]) processBatch() error {
	g, ctx := errgroup.WithContext(bp.ctx)

	for _, item := range bp.items {
		item := item
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic occurred while processing item: %v", r)
				}
			}()
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				result, err := bp.process(item)
				if err != nil {
					return err
				}

				bp.mu.Lock()
				bp.results[item] = result
				bp.mu.Unlock()

				bp.barMu.Lock()
				bp.progressBar.Add(1)
				bp.barMu.Unlock()

				return nil
			}
		})
	}
	return g.Wait()
}

// Create a new batch processor
func newBatchProcessor[K comparable, V any](
	ctx context.Context,
	items []K,
	process func(K) (V, error),
) *batchProcessor[K, V] {
	return &batchProcessor[K, V]{
		ctx:         ctx,
		items:       items,
		process:     process,
		results:     make(map[K]V),
		progressBar: progressbar.Default(int64(len(items))),
	}
}

func (r *ethereumRepository) GetBlockTimestamps(ctx context.Context, blockNumbers []uint64) (map[uint64]time.Time, error) {
	processor := newBatchProcessor(
		ctx,
		blockNumbers,
		func(blockNumber uint64) (time.Time, error) {
			timestamp, err := r.GetBlockTimestamp(ctx, blockNumber)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed to get timestamp for block %d: %w", blockNumber, err)
			}
			return timestamp, nil
		},
	)
	defer processor.progressBar.Finish()

	if err := processor.processBatch(); err != nil {
		return nil, err
	}
	return processor.results, nil
}

func (r *ethereumRepository) GetTransactionSenders(ctx context.Context, txHashes []string) (map[string]string, error) {
	processor := newBatchProcessor(
		ctx,
		txHashes,
		func(txHash string) (string, error) {
			sender, err := r.GetTransactionSender(ctx, txHash)
			if err != nil {
				return "", fmt.Errorf("failed to get sender for transaction %s: %w", txHash, err)
			}
			return sender, nil
		},
	)
	defer processor.progressBar.Finish()

	if err := processor.processBatch(); err != nil {
		return nil, err
	}
	return processor.results, nil
}

func (r *ethereumRepository) CalcTransactionFeeBatch(ctx context.Context, txHashes []string) (map[string]float64, error) {
	processor := newBatchProcessor(
		ctx,
		txHashes,
		func(txHash string) (float64, error) {
			fee, err := r.CalcTransactionFee(ctx, txHash)
			if err != nil {
				return 0, fmt.Errorf("failed to calculate fee for transaction %s: %w", txHash, err)
			}
			return fee, nil
		},
	)
	defer processor.progressBar.Finish()

	if err := processor.processBatch(); err != nil {
		return nil, err
	}
	return processor.results, nil
}
