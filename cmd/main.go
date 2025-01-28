package main

import (
	"bera_indexer/internal/config"
	"bera_indexer/internal/repository"
	"bera_indexer/internal/services"
	"bera_indexer/internal/utils"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/robfig/cron/v3"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error running indexer: %v", err)
	}
}

func run() error {
	config := config.LoadConfig()

	client, err := ethclient.Dial(config.RPC_URL)
	if err != nil {
		return fmt.Errorf("failed to connect to the Ethereum client: %w", err)
	}
	ethereumRepository := repository.NewEthereumRepository(client, config)

	dbRepository, err := repository.ConnectToDb(config)
	if err != nil {
		return fmt.Errorf("failed to connect to the database: %w", err)
	}

	c := cron.New(cron.WithSeconds())
	_, err = c.AddFunc(config.CronSchedule, func() {
		syncIndexer(&dbRepository, &ethereumRepository, config)
		utils.PrintNextExecution(c)
	})

	if err != nil {
		return fmt.Errorf("failed to schedule cron jobs: %w", err)
	}

	syncIndexer(&dbRepository, &ethereumRepository, config)
	c.Start()
	utils.PrintNextExecution(c)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sigChan

	// Cleanup
	log.Println("Shutting down gracefully...")
	c.Stop()
	return nil
}

func syncIndexer(dbRepository *repository.DbRepository, ethereumRepository *repository.EthereumRepository, config *config.Config) {
	syncTransactionsErr := services.SyncTransactions(dbRepository, ethereumRepository, config)
	if syncTransactionsErr != nil {
		log.Fatalf("Error syncing transactions: %v", syncTransactionsErr)
	}
}
