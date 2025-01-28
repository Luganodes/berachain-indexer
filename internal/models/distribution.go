package models

import "time"

type Distribution struct {
	Validator       string    `bson:"validator"`
	Receiver        string    `bson:"receiver"`
	Amount          string    `bson:"amount"`
	TransactionHash string    `bson:"transactionHash"`
	LogIndex        uint      `bson:"logIndex"`
	BlockNumber     uint64    `bson:"blockNumber"`
	BlockTimestamp  time.Time `bson:"blockTimestamp"`
	Fee             float64   `bson:"fee"`
	TransactionFrom string    `bson:"transactionFrom"`
	ToContract      string    `bson:"toContract"`
}
