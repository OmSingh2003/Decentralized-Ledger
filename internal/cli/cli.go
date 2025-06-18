package cli

import (
    "flag"
    "fmt"
    "os"
    "strconv"

    "github.com/OmSingh2003/decentralized-ledger/internal/blockchain"
    "github.com/OmSingh2003/decentralized-ledger/internal/consensus"
    "github.com/OmSingh2003/decentralized-ledger/internal/crypto/pow"
    "github.com/OmSingh2003/decentralized-ledger/internal/transaction"
    "github.com/OmSingh2003/decentralized-ledger/internal/wallet"
)

// CLI responsible for processing command line arguments
type CLI struct {
    bc *blockchain.Blockchain
}

// NewCLI creates a new CLI instance
func NewCLI(bc *blockchain.Blockchain) *CLI {
    return &CLI{bc}
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  createwallet - Creates a new wallet")
	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
	fmt.Println("  printchain - Print all the blocks of the blockchain")
	fmt.Println("  reindexutxo - Rebuilds the UTXO set")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
	fmt.Println("  stake -address ADDRESS -amount AMOUNT - Add stake for PoS validator")
	fmt.Println("  compactdb - Compacts the database to reduce fragmentation and file size")
}

// validateArgs validates command line arguments
func (cli *CLI) validateArgs() {
    if len(os.Args) < 2 {
        cli.printUsage()
        os.Exit(1)
    }
}

// Run parses command line arguments and processes commands
func (cli *CLI) Run() error {
    cli.validateArgs()

	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	stakeCmd := flag.NewFlagSet("stake", flag.ExitOnError)
	compactDBCmd := flag.NewFlagSet("compactdb", flag.ExitOnError)

	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	stakeAddress := stakeCmd.String("address", "", "The address to stake from")
	stakeAmount := stakeCmd.Int64("amount", 0, "Amount to stake")

    switch os.Args[1] {
    case "createwallet":
        err := createWalletCmd.Parse(os.Args[2:])
        if err != nil {
            return err
        }
    case "getbalance":
        err := getBalanceCmd.Parse(os.Args[2:])
        if err != nil {
            return err
        }
    case "listaddresses":
        err := listAddressesCmd.Parse(os.Args[2:])
        if err != nil {
            return err
        }
    case "printchain":
        err := printChainCmd.Parse(os.Args[2:])
        if err != nil {
            return err
        }
    case "reindexutxo":
        err := reindexUTXOCmd.Parse(os.Args[2:])
        if err != nil {
            return err
        }
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			return err
		}
	case "stake":
		err := stakeCmd.Parse(os.Args[2:])
		if err != nil {
			return err
		}
	case "compactdb":
		err := compactDBCmd.Parse(os.Args[2:])
		if err != nil {
			return err
		}
	default:
		cli.printUsage()
		return fmt.Errorf("invalid command")
    }

    if createWalletCmd.Parsed() {
        return cli.createWallet()
    }

    if getBalanceCmd.Parsed() {
        if *getBalanceAddress == "" {
            getBalanceCmd.Usage()
            return fmt.Errorf("address is required")
        }
        return cli.getBalance(*getBalanceAddress)
    }

    if listAddressesCmd.Parsed() {
        return cli.listAddresses()
    }

    if printChainCmd.Parsed() {
        return cli.printChain()
    }

    if reindexUTXOCmd.Parsed() {
        return cli.reindexUTXO()
    }

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			return fmt.Errorf("from, to and amount are required")
		}
		return cli.send(*sendFrom, *sendTo, *sendAmount)
	}

	if stakeCmd.Parsed() {
		if *stakeAddress == "" || *stakeAmount <= 0 {
			stakeCmd.Usage()
			return fmt.Errorf("address and amount are required")
		}
		return cli.addStake(*stakeAddress, *stakeAmount)
	}

	if compactDBCmd.Parsed() {
		return cli.compactDB()
	}

	return nil
}

func (cli *CLI) createWallet() error {
    w := wallet.NewWallet()
    address := w.GetAddress()
    fmt.Printf("Your new address: %s\n", address)
    return nil
}

func (cli *CLI) getBalance(address string) error {
    w := wallet.LoadWallet(address)
    if w == nil {
        return fmt.Errorf("wallet not found for address: %s", address)
    }

    pubKeyHash := wallet.HashPubKey(w.PublicKey)

    UTXOSet := blockchain.UTXOSet{Blockchain: cli.bc}
    UTXOs := UTXOSet.FindUTXO(pubKeyHash)

    balance := 0
    for _, out := range UTXOs {
        balance += out.Value
    }

    fmt.Printf("Balance of '%s': %d\n", address, balance)
    return nil
}

func (cli *CLI) listAddresses() error {
    addresses := wallet.ListAddresses()
    for _, address := range addresses {
        fmt.Println(address)
    }
    return nil
}

func (cli *CLI) printChain() error {
    bci := cli.bc.Iterator()

    for {
        block, err := bci.Next()
        if err != nil {
            return fmt.Errorf("error getting next block: %v", err)
        }
        if block == nil {
            break
        }

        fmt.Printf("============ Block %x ============\n", block.Hash)
        fmt.Printf("Prev. block: %x\n", block.PrevBlockHash)
        
        // Check if this is a PoS block (has validator signature)
        if len(block.GetValidatorPubKey()) > 0 {
            fmt.Printf("PoS Block - Validator: %x\n", block.GetValidatorPubKey())
            fmt.Printf("Signature: %x\n", block.GetSignature())
        } else {
            // This is a PoW block
            powCheck := pow.NewProofOfWork(block, block.GetBits())
            fmt.Printf("PoW: %s\n", strconv.FormatBool(powCheck.Validate()))
        }
        fmt.Println()

        for _, tx := range block.Transactions {
            fmt.Println(tx)
        }
        fmt.Printf("\n\n")

        if len(block.PrevBlockHash) == 0 {
            break
        }
    }
    return nil
}

func (cli *CLI) reindexUTXO() error {
    UTXOSet := blockchain.UTXOSet{Blockchain: cli.bc}
    err := UTXOSet.Reindex()
    if err != nil {
        return fmt.Errorf("failed to reindex UTXO: %v", err)
    }
    
    count := len(UTXOSet.FindUTXO(nil))
    fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
    return nil
}

func (cli *CLI) send(from, to string, amount int) error {
    fromWallet := wallet.LoadWallet(from)
    if fromWallet == nil {
        return fmt.Errorf("wallet not found for address: %s", from)
    }

    toWallet := wallet.LoadWallet(to)
    if toWallet == nil {
        return fmt.Errorf("wallet not found for address: %s", to)
    }

    UTXOSet := blockchain.UTXOSet{Blockchain: cli.bc}

    tx, err := transaction.NewUTXOTransaction(fromWallet, wallet.HashPubKey(toWallet.PublicKey), amount, UTXOSet.FindSpendableOutputs)
    if err != nil {
        return fmt.Errorf("failed to create transaction: %v", err)
    }

    // Sign the transaction
    err = cli.bc.SignTransaction(tx, fromWallet)
    if err != nil {
        return fmt.Errorf("failed to sign transaction: %v", err)
    }

    cbTx := transaction.NewCoinbaseTx(fromWallet.PublicKey, "")
    txs := []*transaction.Transaction{cbTx, tx}

	newBlock, err := cli.bc.MineBlock(txs, fromWallet)
	if err != nil {
		return fmt.Errorf("failed to mine new block: %v", err)
	}

    err = UTXOSet.Update(newBlock)
    if err != nil {
        return fmt.Errorf("failed to update UTXO set: %v", err)
    }

    fmt.Println("Success!")
    return nil
}

// addStake adds stake for a PoS validator
func (cli *CLI) addStake(address string, amount int64) error {
	w := wallet.LoadWallet(address)
	if w == nil {
		return fmt.Errorf("wallet not found for address: %s", address)
	}

	// Get the PoS consensus instance from blockchain
	posConsensus, ok := cli.bc.GetConsensus().(*consensus.PoSConsensus)
	if !ok {
		return fmt.Errorf("blockchain is not using PoS consensus")
	}

	// Add stake for the validator
	err := posConsensus.AddStake(amount, w)
	if err != nil {
		return fmt.Errorf("failed to add stake: %v", err)
	}

	fmt.Printf("Successfully added stake of %d for validator %s\n", amount, address)
	return nil
}

// compactDB compacts the blockchain database to reduce fragmentation
func (cli *CLI) compactDB() error {
	if cli.bc == nil {
		return fmt.Errorf("blockchain is not initialized")
	}

	fmt.Println("Compacting blockchain database...")
	err := cli.bc.CompactDB()
	if err != nil {
		return fmt.Errorf("failed to compact database: %v", err)
	}

	fmt.Println("Database compaction completed successfully.")
	return nil
}
