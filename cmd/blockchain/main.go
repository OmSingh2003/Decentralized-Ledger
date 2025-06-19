package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    
    "github.com/OmSingh2003/decentralized-ledger/internal/blockchain"
    "github.com/OmSingh2003/decentralized-ledger/internal/cli"
    "github.com/OmSingh2003/decentralized-ledger/internal/wallet"
)

func main() {
    // Check for commands that don't require blockchain initialization
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "createwallet":
            // Create a new wallet
            w := wallet.NewWallet()
            address := w.GetAddress()
            fmt.Printf("Your new address: %s\n", address)
            return
            
        case "listaddresses":
            // List all wallet addresses
            addresses := wallet.ListAddresses()
            for _, address := range addresses {
                fmt.Println(address)
            }
            return
            
        case "init":
            // Initialize blockchain with genesis block
            initCmd := flag.NewFlagSet("init", flag.ExitOnError)
            initAddress := initCmd.String("address", "", "The address to use for mining the genesis block")
            
            if err := initCmd.Parse(os.Args[2:]); err != nil {
                log.Fatalf("Failed to parse init command: %v", err)
            }
            
            if *initAddress == "" {
                fmt.Println("Error: Address is required")
                fmt.Println("Usage: blockchain init -address WALLET_ADDRESS")
                return
            }
            
            // Validate wallet exists
            if !wallet.ValidateAddress(*initAddress) {
                fmt.Printf("Error: Invalid address format: %s\n", *initAddress)
                return
            }
            
            // Create blockchain with genesis block
            bc, err := createBlockchain(*initAddress)
            if err != nil {
                log.Fatalf("Failed to create blockchain: %v", err)
            }
            defer bc.CloseDB()
            
            fmt.Println("Blockchain initialized with genesis block!")
            return
        }
    }
    
    // For all other commands, initialize blockchain
    bc, err := blockchain.NewBlockchain()
    if err != nil {
        log.Fatalf("Failed to create blockchain: %v", err)
    }
    
    // Ensure database is closed properly when main exits
    defer func() {
        if err := bc.CloseDB(); err != nil {
            log.Printf("Error closing database: %v", err)
        }
    }()
    
    // Initialize and run CLI
    cli := cli.NewCLI(bc)
    if err := cli.Run(); err != nil {
        log.Fatalf("CLI error: %v", err)
    }
}

// createBlockchain creates a new blockchain with a genesis block and rewards the miner
func createBlockchain(minerAddress string) (*blockchain.Blockchain, error) {
    // Validate the miner address format
    if !wallet.ValidateAddress(minerAddress) {
        return nil, fmt.Errorf("invalid address format: %s", minerAddress)
    }
    
    // Create a new blockchain with the genesis block
    // Note: The actual wallet loading with password will be handled by the CLI commands
    bc, err := blockchain.CreateBlockchain(nil) // Pass nil for now, will be updated later
    if err != nil {
        return nil, fmt.Errorf("failed to create blockchain: %v", err)
    }
    
    return bc, nil
}
