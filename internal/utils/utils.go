package utils

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/robfig/cron/v3"
)

func ConvertWeiToEther(wei *big.Int) float64 {
	etherValue := new(big.Float).SetInt(wei)
	etherValue.Quo(etherValue, big.NewFloat(1e18)) // Divide by 10^18
	result, _ := etherValue.Float64()              // Convert big.Float to float64
	return result
}

func RetryWithBackoff(ctx context.Context, retries int, fn func() error) error {
	var err error
	for i := 0; i < retries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return fmt.Errorf("failed after %d retries: %v", retries, err)
}

func PrintNextExecution(c *cron.Cron) {
	entries := c.Entries()
	if len(entries) > 0 {
		nextRun := entries[0].Next
		log.Printf("Next cron execution scheduled for: %v", nextRun)
	}
}
