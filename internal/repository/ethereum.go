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
	GetRewardAllocation(ctx context.Context, pubkey string) ([]common.Address, error)
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

type batchProcessor[K comparable, V any] struct {
	ctx                context.Context
	items              []K
	process            func(K) (V, error)
	results            map[K]V
	mu                 sync.Mutex
	progressBar        *progressbar.ProgressBar
	barMu              sync.Mutex
	concurrentRequests int
}

func (bp *batchProcessor[K, V]) processBatch() error {
	g, ctx := errgroup.WithContext(bp.ctx)
	g.SetLimit(bp.concurrentRequests)

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

func newBatchProcessor[K comparable, V any](
	ctx context.Context,
	items []K,
	concurrentRequests int,
	process func(K) (V, error),
) *batchProcessor[K, V] {
	return &batchProcessor[K, V]{
		ctx:                ctx,
		items:              items,
		process:            process,
		results:            make(map[K]V),
		progressBar:        progressbar.Default(int64(len(items))),
		concurrentRequests: concurrentRequests,
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
	var startBlocks []uint64
	batchSize := r.config.GetLogsBatchSize

	for start := startBlock; start <= endBlock; start += batchSize {
		startBlocks = append(startBlocks, start)
	}

	processor := newBatchProcessor(
		ctx,
		startBlocks,
		r.config.ConcurrentRequests,
		func(start uint64) ([]types.Log, error) {
			end := min(start+batchSize-1, endBlock)
			query := ethereum.FilterQuery{
				Addresses: contracts,
				Topics:    topics,
				FromBlock: big.NewInt(int64(start)),
				ToBlock:   big.NewInt(int64(end)),
			}

			operation := func() ([]types.Log, error) {
				logs, err := r.client.FilterLogs(ctx, query)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch logs from block %d to %d: %w", start, end, err)
				}
				return logs, nil
			}
			return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
		},
	)
	defer processor.progressBar.Finish()

	if err := processor.processBatch(); err != nil {
		return nil, err
	}

	var allLogs []types.Log
	for _, logs := range processor.results {
		allLogs = append(allLogs, logs...)
	}
	return allLogs, nil
}

func (r *ethereumRepository) GetBlockTimestamps(ctx context.Context, blockNumbers []uint64) (map[uint64]time.Time, error) {
	processor := newBatchProcessor(
		ctx,
		blockNumbers,
		r.config.ConcurrentRequests,
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
		r.config.ConcurrentRequests,
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
		r.config.ConcurrentRequests,
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

func (r *ethereumRepository) GetRewardAllocation(ctx context.Context, pubkey string) ([]common.Address, error) {
	data, err := r.config.Contracts.BerachefContract.ABI.Pack("getActiveRewardAllocation", common.FromHex(pubkey))
	if err != nil {
		return nil, fmt.Errorf("failed to pack getActiveRewardAllocation: %w", err)
	}
	msg := ethereum.CallMsg{
		To:   &r.config.Contracts.BerachefContract.Address,
		Data: data,
	}

	operation := func() ([]common.Address, error) {
		response, err := r.client.CallContract(ctx, msg, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get reward allocation: %w", err)
		}
		decoded, err := r.config.Contracts.BerachefContract.ABI.Unpack("getActiveRewardAllocation", response)
		if err != nil {
			return nil, fmt.Errorf("failed to unpack getActiveRewardAllocation: %w", err)
		}
		decodedStruct := decoded[0].(struct {
			StartBlock uint64 `json:"startBlock"`
			Weights    []struct {
				Receiver            common.Address `json:"receiver"`
				PercentageNumerator *big.Int       `json:"percentageNumerator"`
			} `json:"weights"`
		})
		receivers := make([]common.Address, len(decodedStruct.Weights))
		for i, weight := range decodedStruct.Weights {
			receivers[i] = weight.Receiver
		}
		return receivers, nil
	}
	return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}
