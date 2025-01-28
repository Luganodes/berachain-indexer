package repository

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/models"
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DbRepository interface {
	Health() error
	Disconnect() error
	FindLastBlockProcessed(ctx context.Context) (uint64, error)
	InsertBlock(ctx context.Context, blockNumber uint64) error
	DoesTransactionExist(ctx context.Context, collectionName string, txHash string, logIndex uint) (bool, error)
	AddBlockReward(ctx context.Context, blockReward models.BlockReward) error
	AddDistribution(ctx context.Context, distribution models.Distribution) error
	AddIncentive(ctx context.Context, incentive models.Incentive) error
	AddDeposit(ctx context.Context, deposit models.Deposit) error
	UpsertDelegator(ctx context.Context, delegator string, validator string, stakedAmount string) error
}

type mongoRepository struct {
	client *mongo.Client
	dbName string
}

func ConnectToDb(config *config.Config) (DbRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	host := config.Db.Host
	port := config.Db.Port
	user := config.Db.User
	password := config.Db.Password
	dbName := config.Db.DbName

	uri := fmt.Sprintf("mongodb://%s:%d", host, port)
	if user != "" && password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d", user, password, host, port)
	}

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}
	log.Println("✅ Db connected")

	return &mongoRepository{
		client: client,
		dbName: dbName,
	}, nil
}

func (r *mongoRepository) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return r.client.Ping(ctx, nil)
}

func (r *mongoRepository) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return r.client.Disconnect(ctx)
}

func (r *mongoRepository) FindLastBlockProcessed(ctx context.Context) (uint64, error) {
	var result models.Metadata
	opts := options.FindOne().SetSort(bson.D{{Key: "lastBlockProcessed", Value: -1}})
	if err := r.Collection("metadata").FindOne(ctx, bson.M{}, opts).Decode(&result); err != nil {
		return 0, err
	}
	return result.LastBlockProcessed, nil
}

func (r *mongoRepository) InsertBlock(ctx context.Context, blockNumber uint64) error {
	return r.Collection("metadata").InsertOne(ctx, models.Metadata{LastBlockProcessed: blockNumber})
}

func (r *mongoRepository) DoesTransactionExist(ctx context.Context, collectionName string, txHash string, logIndex uint) (bool, error) {
	result := r.Collection(collectionName).FindOne(ctx, bson.M{"transactionHash": txHash, "logIndex": logIndex})
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, fmt.Errorf("error checking transaction existence: %v", result.Err())
	}
	return true, nil
}

func (r *mongoRepository) AddBlockReward(ctx context.Context, blockReward models.BlockReward) error {
	return r.Collection("blockRewards").InsertOne(ctx, blockReward)
}

func (r *mongoRepository) AddDistribution(ctx context.Context, distribution models.Distribution) error {
	return r.Collection("distributions").InsertOne(ctx, distribution)
}

func (r *mongoRepository) AddIncentive(ctx context.Context, incentive models.Incentive) error {
	return r.Collection("incentives").InsertOne(ctx, incentive)
}

func (r *mongoRepository) AddDeposit(ctx context.Context, deposit models.Deposit) error {
	return r.Collection("deposits").InsertOne(ctx, deposit)
}

func (r *mongoRepository) UpsertDelegator(ctx context.Context, delegatorAddress string, validator string, stakedAmount string) error {
	filter := bson.M{"delegator": delegatorAddress, "validator": validator}

	var delegator models.Delegator
	err := r.Collection("delegators").FindOne(ctx, filter).Decode(&delegator)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return r.Collection("delegators").InsertOne(ctx, models.Delegator{Delegator: delegatorAddress, Validator: validator, StakedAmount: stakedAmount})
		}
		return fmt.Errorf("error fetching delegation: %v", err)
	}

	currentStaked, ok := new(big.Int).SetString(delegator.StakedAmount, 10)
	if !ok {
		return fmt.Errorf("failed to convert staked amount to big.Int for delegator %s: %v", delegatorAddress, delegator.StakedAmount)
	}
	additionalStake, ok := new(big.Int).SetString(stakedAmount, 10)
	if !ok {
		return fmt.Errorf("failed to convert staked amount to big.Int for delegator %s: %v", delegatorAddress, stakedAmount)
	}
	updatedStaked := currentStaked.Add(currentStaked, additionalStake)
	return r.Collection("delegators").UpdateOne(ctx, filter, bson.M{"stakedAmount": updatedStaked.String()})
}
