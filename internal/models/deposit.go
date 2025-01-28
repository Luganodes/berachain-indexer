package models

import "time"

type Deposit struct {
	Validator             string    `bson:"validator"`
	Amount                string    `bson:"amount"`
	WithdrawalCredentials string    `bson:"withdrawalCredentials"`
	Signature             string    `bson:"signature"`
	TransactionHash       string    `bson:"transactionHash"`
	LogIndex              uint      `bson:"logIndex"`
	BlockNumber           uint64    `bson:"blockNumber"`
	BlockTimestamp        time.Time `bson:"blockTimestamp"`
	Fee                   float64   `bson:"fee"`
	TransactionFrom       string    `bson:"transactionFrom"`
	ToContract            string    `bson:"toContract"`
}
