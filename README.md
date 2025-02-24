<a name="readme-top"></a>
[![Banner](./imgs/banner.png)](https://github.com/Luganodes/Solana-Indexer)

# Berachain Indexer

A high-performance, ultra-efficient indexer for Berachain, designed to seamlessly track validator deposits, block rewards, reward distributions, and incentives from reward vaults. Optimized for speed, it can index at rates of up to 50,000 blocks per second—or even faster—depending on the RPC provider. With built-in support for multiple validators, it ensures accurate and reliable tracking at scale.

<br />
<div align="center">
    <a href="https://github.com/Luganodes/berachain-indexer/issues">Report Bug</a>
    |
    <a href="https://github.com/Luganodes/berachain-indexer/issues">Request Feature</a>
</div>

## Schema Definitions

### Deposit Schema

| Field                 | Type      | Description                                    |
| --------------------- | --------- | ---------------------------------------------- |
| Validator             | string    | Public key of the validator                    |
| Amount                | string    | Deposit amount                                 |
| WithdrawalCredentials | string    | Withdrawal credentials of the validator        |
| Signature             | string    | Transaction signature                          |
| TransactionHash       | string    | Transaction hash                               |
| LogIndex              | uint      | Index of the log in the block                  |
| BlockNumber           | uint64    | Block number in which transaction was included |
| BlockTimestamp        | time.Time | Timestamp of the block                         |
| Fee                   | float64   | Transaction fee                                |
| TransactionFrom       | string    | Address that initiated the transaction         |
| ToContract            | string    | Contract address receiving the transaction     |

### Delegator Schema

| Field        | Type   | Description                 |
| ------------ | ------ | --------------------------- |
| Delegator    | string | Address of the delegator    |
| Validator    | string | Public key of the validator |
| StakedAmount | string | Amount staked by delegator  |

### Block Reward Schema

| Field           | Type      | Description                                    |
| --------------- | --------- | ---------------------------------------------- |
| Validator       | string    | Validator ID                                   |
| BaseRate        | string    | Base Emission                                  |
| RewardRate      | string    | Reward Vault Emission                          |
| TransactionHash | string    | Transaction hash                               |
| LogIndex        | uint      | Index of the log in the block                  |
| BlockNumber     | uint64    | Block number in which transaction was included |
| BlockTimestamp  | time.Time | Timestamp of the block                         |
| Fee             | float64   | Transaction fee                                |
| TransactionFrom | string    | Address that initiated the transaction         |
| ToContract      | string    | Contract address receiving the transaction     |

### Distribution Schema

| Field           | Type      | Description                                           |
| --------------- | --------- | ----------------------------------------------------- |
| Validator       | string    | Validator ID                                          |
| Receiver        | string    | Rewrard vault to which reward emission is distributed |
| Amount          | string    | Reward emission amount                                |
| TransactionHash | string    | Transaction hash                                      |
| LogIndex        | uint      | Index of the log in the block                         |
| BlockNumber     | uint64    | Block number in which transaction was included        |
| BlockTimestamp  | time.Time | Timestamp of the block                                |
| Fee             | float64   | Transaction fee                                       |
| TransactionFrom | string    | Address that initiated the transaction                |
| ToContract      | string    | Contract address receiving the transaction            |

### Incentive Schema

| Field           | Type      | Description                                    |
| --------------- | --------- | ---------------------------------------------- |
| Validator       | string    | Validator ID                                   |
| Token           | string    | Incentive token address                        |
| BGTEmitted      | string    | Amount of BGT tokens emitted                   |
| Amount          | string    | Incentive amount                               |
| TransactionHash | string    | Transaction hash                               |
| LogIndex        | uint      | Index of the log in the block                  |
| BlockNumber     | uint64    | Block number in which transaction was included |
| BlockTimestamp  | time.Time | Timestamp of the block                         |
| TransactionFrom | string    | Address that initiated the transaction         |
| Fee             | float64   | Transaction fee                                |
| ToContract      | string    | Contract address receiving the transaction     |

### Metadata Schema

| Field              | Type   | Description                                     |
| ------------------ | ------ | ----------------------------------------------- |
| LastBlockProcessed | uint64 | Indicates the next block to start the cron from |

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes.

### Prerequisites

- [Go](https://go.dev/doc/install)

- [MongoDB](https://www.mongodb.com/docs/manual/installation/)

- Docker (Optional)
  - For macOS: [Download Docker Desktop for Mac](https://docs.docker.com/desktop/mac/install/)
  - For Windows: [Download Docker Desktop for Windows](https://docs.docker.com/desktop/windows/install/)
  - For Linux: [Docker for Linux](https://docs.docker.com/engine/install/)

### Local Setup

1. Clone the repo

   ```sh
   git clone https://github.com/Luganodes/berachain-indexer.git
   cd berachain-indexer
   cp .env.sample .env
   ```

2. Popluate .env with appropriate values. Look at [.env.sample](./.env.sample) for reference.

### MakeFile

Build the application

```bash
make build
```

Run the application

```bash
make run
```

Docker run

```bash
make docker-run
```

Shutdown docker containers

```bash
make docker-down
```

Live reload the application

```bash
make watch
```

Clean up binary from the last build

```bash
make clean
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feat/AmazingFeature`)
3. Make some amazing changes.
4. `git add .`
5. Commit your Changes (`git commit -m "<Verb>: <Action>"`)
6. Push to the Branch (`git push origin feat/AmazingFeature`)
7. Open a Pull Request

To start contributing, check out [`CONTRIBUTING.md`](./CONTRIBUTING.md) . New contributors are always welcome to support this project.

## License

Distributed under the MIT License. See [`LICENSE`](./LICENSE) for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>
