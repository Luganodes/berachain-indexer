package models

import "time"

type Incentive struct {
	Validator       string    `bson:"validator"`
	Token           string    `bson:"token"`
	BGTEmitted      string    `bson:"bgtEmitted"`
	Amount          string    `bson:"amount"`
	TransactionHash string    `bson:"transactionHash"`
	LogIndex        uint      `bson:"logIndex"`
	BlockNumber     uint64    `bson:"blockNumber"`
	BlockTimestamp  time.Time `bson:"blockTimestamp"`
	TransactionFrom string    `bson:"transactionFrom"`
	Fee             float64   `bson:"fee"`
	ToContract      string    `bson:"toContract"`
}
