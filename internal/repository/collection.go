package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
)

type CollectionOperations interface {
	InsertOne(ctx context.Context, document interface{}) error
	BulkWrite(ctx context.Context, operations []mongo.WriteModel, opts *options.BulkWriteOptions) (*mongo.BulkWriteResult, error)
	FindOne(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) *mongo.SingleResult
	FindMany(ctx context.Context, filter bson.M, opts *options.FindOptions, documents interface{}) error
	UpdateOne(ctx context.Context, filter bson.M, update bson.M) error
}

type mongoCollection struct {
	coll *mongo.Collection
}

func (r *mongoRepository) Collection(name string) CollectionOperations {
	return &mongoCollection{
		coll: r.client.Database(r.dbName).Collection(name),
	}
}

func (c *mongoCollection) InsertOne(ctx context.Context, document interface{}) error {
	doc, ok := document.(map[string]interface{})
	if !ok {
		docBytes, err := bson.Marshal(document)
		if err != nil {
			return fmt.Errorf("failed to marshal document: %v", err)
		}
		if err := bson.Unmarshal(docBytes, &doc); err != nil {
			return fmt.Errorf("failed to unmarshal document: %v", err)
		}
	}

	now := time.Now()
	doc["created_at"] = now
	doc["updated_at"] = now
	if _, err := c.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("failed to insert document: %v", err)
	}
	return nil
}

type BulkWriteError struct {
	InsertedCount int64
	MatchedCount  int64
	ModifiedCount int64
	DeletedCount  int64
	UpsertedCount int64
	UpsertedIDs   map[int64]interface{}
	Err           error
}

func (r *BulkWriteError) Error() string {
	return r.Err.Error()
}

func (c *mongoCollection) BulkWrite(ctx context.Context, operations []mongo.WriteModel, opts *options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	if len(operations) == 0 {
		return &mongo.BulkWriteResult{
			InsertedCount: 0,
			MatchedCount:  0,
			ModifiedCount: 0,
			DeletedCount:  0,
			UpsertedCount: 0,
			UpsertedIDs:   make(map[int64]interface{}),
		}, nil
	}
	const batchSize = 1000 // MongoDB recommended batch size

	g, ctx := errgroup.WithContext(ctx)
	results := make([]*mongo.BulkWriteResult, (len(operations)+batchSize-1)/batchSize)

	for i := 0; i < len(operations); i += batchSize {
		batchIndex := i / batchSize
		end := min(i+batchSize, len(operations))
		batch := operations[i:end]

		g.Go(func() error {
			// Check if context was cancelled by another goroutine's error
			if ctx.Err() != nil {
				return ctx.Err()
			}

			operation := func() (*mongo.BulkWriteResult, error) {
				return c.coll.BulkWrite(ctx, batch, opts)
			}

			result, err := backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
			if err != nil {
				return fmt.Errorf("failed to execute bulk write batch: %v", err)
			}
			results[batchIndex] = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		bulkWriteError := &BulkWriteError{
			Err: err,
		}
		for _, result := range results {
			if result != nil {
				bulkWriteError.InsertedCount += result.InsertedCount
				bulkWriteError.MatchedCount += result.MatchedCount
				bulkWriteError.ModifiedCount += result.ModifiedCount
				bulkWriteError.DeletedCount += result.DeletedCount
				bulkWriteError.UpsertedCount += result.UpsertedCount
			}
		}
		return nil, bulkWriteError
	}

	// Initialize finalResult with a non-nil map
	finalResult := &mongo.BulkWriteResult{
		UpsertedIDs: make(map[int64]interface{}),
	}
	for _, result := range results {
		finalResult.InsertedCount += result.InsertedCount
		finalResult.MatchedCount += result.MatchedCount
		finalResult.ModifiedCount += result.ModifiedCount
		finalResult.DeletedCount += result.DeletedCount
		finalResult.UpsertedCount += result.UpsertedCount
		for k, v := range result.UpsertedIDs {
			finalResult.UpsertedIDs[k] = v
		}
	}
	return finalResult, nil
}

func (c *mongoCollection) FindOne(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) *mongo.SingleResult {
	return c.coll.FindOne(ctx, filter, opts...)
}

func (c *mongoCollection) FindMany(ctx context.Context, filter bson.M, opts *options.FindOptions, documents interface{}) error {
	cursor, err := c.coll.Find(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("failed to find documents: %v", err)
	}
	if err := cursor.All(ctx, documents); err != nil {
		return fmt.Errorf("failed to decode documents: %v", err)
	}
	return nil
}

func (c *mongoCollection) UpdateOne(ctx context.Context, filter bson.M, update bson.M) error {
	finalUpdate := bson.M{
		"$set": update,
		"$currentDate": bson.M{
			"updated_at": true,
		},
	}

	if _, err := c.coll.UpdateOne(ctx, filter, finalUpdate); err != nil {
		return fmt.Errorf("failed to update document: %v", err)
	}
	return nil
}
