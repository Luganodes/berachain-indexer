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

type Validator struct {
	Id     string
	Pubkey string
}

type Config struct {
	Db         DbConfig
	RPC_URL    string
	StartBlock uint64

	Contracts Contracts
	Events    map[string]string

	Validators []Validator

	GetLogsBatchSize     uint64
	ProcessLogsBatchSize uint64
	CronSchedule         string
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

		Validators: loadValidators(),

		StartBlock:           uint64((getEnvInt("START_BLOCK", ptr(0)))),
		GetLogsBatchSize:     uint64(getEnvInt("GET_LOGS_BATCH_SIZE", ptr(5000))),         // adjust depending on your RPC limiations
		ProcessLogsBatchSize: uint64(getEnvInt("PROCESS_LOGS_BATCH_SIZE", ptr(5000*200))), // too much at once will cause memory issues
		CronSchedule:         getEnvString("CRON_SCHEDULE", ptr("0 5 * * * *")),
	}
	log.Println("✅ Config Loaded")
	return &config
}

func loadValidators() []Validator {
	validatorStr := getEnvString("VALIDATORS", nil)

	var validators []Validator
	pairs := strings.Split(validatorStr, ",")
	for _, pair := range pairs {
		parts := strings.Split(pair, ":")
		if len(parts) != 2 {
			panic(fmt.Sprintf("Invalid validator format in env. Expected '<pubkey>:<validator_id>', got '%s'", pair))
		}
		validators = append(validators, Validator{
			Pubkey: parts[0],
			Id:     parts[1],
		})
	}
	return validators
}

func loadContracts() Contracts {

	depositContract := getEnvString("DEPOSIT_CONTRACT", ptr("0x4242424242424242424242424242424242424242"))
	depositABI, err := readABI("abi/deposit.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	blockRewardContract := getEnvString("BLOCK_REWARD_CONTRACT", ptr("0x1ae7dd7ae06f6c58b4524d9c1f816094b1bccd8e"))
	blockRewardABI, err := readABI("abi/blockReward.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	distributionContract := getEnvString("DISTRIBUTION_CONTRACT", ptr("0xd2f19a79b026fb636a7c300bf5947df113940761"))
	distributionABI, err := readABI("abi/distribution.json")
	if err != nil {
		panic(fmt.Sprintf("Error reading ABI: %v", err))
	}

	vaultContracts := strings.Split(getEnvString("VAULT_CONTRACTS", ptr("0x086f82fa0ca310cc835a9db4f53697687ef149c7,0x17376ad6167a5592fbeaa42e6068c132474a513d,0x6649bc987a7c0fb0199c523de1b1b330cd0457a8,0xc2baa8443cda8ebe51a640905a8e6bc4e1f9872c,0xf99be47baf0c22b7eb5eac42c8d91b9942dc7e84")), ",")
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
