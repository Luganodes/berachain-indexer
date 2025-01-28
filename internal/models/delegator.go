package models

type Delegator struct {
	Delegator    string `bson:"delegator"`
	Validator    string `bson:"validator"`
	StakedAmount string `bson:"stakedAmount"`
}
