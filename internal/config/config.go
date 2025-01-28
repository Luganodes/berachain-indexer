package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
)

type Contract struct {
	Address common.Address
	ABI     abi.ABI
}

type Contracts struct {
	DepositContract      Contract
	BlockRewardContract  Contract
	DistributionContract Contract
	VaultContracts       []common.Address
	VaultContractAbi     abi.ABI
}

type Config struct {
	Db         DbConfig
	RPC_URL    string
	StartBlock uint64

	Contracts Contracts
	Events    map[string]string

	ValidatorId     string
	ValidatorPubkey string

	BatchSize         uint64
	ConcurrentBatches uint64
	MaxRetries        int
	CronSchedule      string
}

type DbConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DbName   string
}

func LoadConfig() *Config {
	if err := LoadEnv(); err != nil {
		panic(fmt.Sprintf("Error loading environment variables: %v", err))
	}

	config := Config{
		Db: DbConfig{
			Host:     getEnvString("DB_HOST", ptr("localhost")),
			User:     getEnvString("DB_USER", ptr("")),
			Password: getEnvString("DB_PASS", ptr("")),
			DbName:   getEnvString("DB_NAME", ptr("bera_indexer")),
			Port:     getEnvInt("DB_PORT", ptr(27017)),
		},
		RPC_URL: getEnvString("RPC_URL", nil),

		Contracts: loadContracts(),
		Events: map[string]string{
			"0x68af751683498a9f9be59fe8b0d52a64dd155255d85cdb29fea30b1e3f891d46": "Deposit",
			"0x71458adf598dc170bd3c4161d44819d254109dd7958425c0fb21acc26d6ca69c": "BlockRewardProcessed",
			"0x027042b00b5da1362792832f3775452610369da8ce2c07af183cdabd276e3a11": "Distributed",
			"0x5468188b6036c5311a3f18fc548c42ccb48a0cdcb9d339e0b2ba38aed4fae36d": "IncentivesProcessed",
		},

		StartBlock:      uint64((getEnvInt("START_BLOCK", ptr(502415)))),
		ValidatorId:     getEnvString("VALIDATOR_ID", ptr("0x23d16af92704d442bc300aa31299866de14a6288681110f6c1ffc1f0f276881e")),
		ValidatorPubkey: getEnvString("VALIDATOR_PUBKEY", ptr("0xb466bd0d3587e98a22f21d0fbc3b24c0a71d00abd58a1303440f94f6f6d91651dc9044534f44cd86994552d2700c56f5")),

		BatchSize:         uint64(getEnvInt("BATCH_SIZE", ptr(5000))),
		ConcurrentBatches: uint64(getEnvInt("CONCURRENT_BATCHES", ptr(15))),
		MaxRetries:        getEnvInt("MAX_RETRIES", ptr(3)),
		CronSchedule:      getEnvString("CRON_SCHEDULE", ptr("0 5 * * * *")),
	}
	log.Println("✅ Config Loaded")
	return &config
}

func loadContracts() Contracts {

	depositContract := getEnvString("DEPOSIT_CONTRACT", ptr("0x4242424242424242424242424242424242424242"))
	depositABI, err := readABI("abi/deposit.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	blockRewardContract := getEnvString("BLOCK_REWARD_CONTRACT", ptr("0x25A37b8E0a090Aa952F037B8534ace17AC3DbC60"))
	blockRewardABI, err := readABI("abi/blockReward.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	distributionContract := getEnvString("DISTRIBUTION_CONTRACT", ptr("0x211bE45338B7C6d5721B5543Eb868547088Aca39"))
	distributionABI, err := readABI("abi/distribution.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	vaultContracts := strings.Split(getEnvString("VAULT_CONTRACTS", ptr("0xb930dcbfb60b5599836f7ab4b7053fb4d881940e")), ",")
	vaultContractAddresses := make([]common.Address, len(vaultContracts))
	for i, contract := range vaultContracts {
		vaultContractAddresses[i] = common.HexToAddress(contract)
	}
	vaultContractAbi, err := readABI("abi/vault.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	contracts := Contracts{
		DepositContract: Contract{
			Address: common.HexToAddress(depositContract),
			ABI:     depositABI,
		},
		BlockRewardContract: Contract{
			Address: common.HexToAddress(blockRewardContract),
			ABI:     blockRewardABI,
		},
		DistributionContract: Contract{
			Address: common.HexToAddress(distributionContract),
			ABI:     distributionABI,
		},
		VaultContracts:   vaultContractAddresses,
		VaultContractAbi: vaultContractAbi,
	}
	return contracts
}

func getConfigPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("error getting current file path")
	}
	return filepath.Dir(filename), nil
}

func LoadEnv() error {
	dir, err := getConfigPath()
	if err != nil {
		return err
	}

	envPath := filepath.Join(dir, "../../.env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// .env file doesn't exist, just return without an error
		return nil
	}

	return godotenv.Load(envPath)
}

func readABI(filePath string) (abi.ABI, error) {
	dir, err := getConfigPath()
	if err != nil {
		return abi.ABI{}, err
	}

	abiPath := filepath.Join(dir, filePath)
	abiFile, err := os.ReadFile(abiPath)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to read ABI file: %v", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(string(abiFile)))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to parse ABI: %v", err)
	}
	return contractABI, nil
}

func getEnvString(key string, defaultValue *string) string {
	value := os.Getenv(key)

	if value != "" {
		return value
	}
	if defaultValue == nil {
		panic(fmt.Sprintf("Environment variable %s is required", key))
	}
	return *defaultValue
}

func getEnvInt(key string, defaultValue *int) int {
	value := os.Getenv(key)
	if value != "" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			panic(fmt.Sprintf("Environment variable %s is not a valid integer", key))
		}
		return intValue
	}
	if defaultValue == nil {
		panic(fmt.Sprintf("Environment variable %s is required", key))
	}
	return *defaultValue
}

func ptr[T any](v T) *T {
	return &v
}
