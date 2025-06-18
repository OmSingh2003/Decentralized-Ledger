package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"sync"

	"github.com/OmSingh2003/decentralized-ledger/internal/block"
	"github.com/OmSingh2003/decentralized-ledger/internal/consensus"
	"github.com/OmSingh2003/decentralized-ledger/internal/transaction"
	"github.com/OmSingh2003/decentralized-ledger/internal/wallet"
	"go.etcd.io/bbolt"
)

const (
	dbFile              = "blockchain.db"
	blocksBucket        = "blocks"
	lastHashKey         = "l" // Key for storing the last block hash
	genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"
	// New constrains for difficulty adjustment
	TARGET_BLOCK_TIME_SECONDS    = 600  // 10 minutes per block
	DIFFICULTY_ADJUSTMENT_BLOCKS = 2025 // Adjust difficulty every 2025 blocks
	MAX_ADJUSTMENT_FACTOR        = 4    // Limit difficulty to 4X (1/4 or 4X )
	INITIAL_TARGET_BITS          = 24   // Starting difficulty for genesis block
)

// Blockchain represents the blockchain structure
type Blockchain struct {
	tip       []byte                // Hash of the latest block
	db        *bbolt.DB             // Database connection
	consensus consensus.Consensus    // Consensus mechanism (PoW or PoS)
	mu        sync.RWMutex          // Mutex for thread safety
}

// BlockchainIterator is used to iterate over blockchain blocks
type BlockchainIterator struct {
	currentHash []byte
	db          *bbolt.DB
}

// NewBlockchain opens an existing blockchain with PoS consensus
func NewBlockchain() (*Blockchain, error) {
	// Only open existing blockchain
	if !DbExists() {
		return nil, fmt.Errorf("no existing blockchain found")
	}

	db, err := bbolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot open blockchain db: %v", err)
	}

	var tip []byte
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return fmt.Errorf("no existing blockchain found")
		}
		tip = b.Get([]byte(lastHashKey))
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	// Use PoS consensus by default
	posConsensus := consensus.NewPoSConsensus(db)
	bc := Blockchain{tip, db, posConsensus, sync.RWMutex{}}
	return &bc, nil
}

// CreateBlockchain creates a new blockchain with a genesis block using PoS
func CreateBlockchain(minerWallet *wallet.Wallet) (*Blockchain, error) {
	// Check if blockchain already exists
	if DbExists() {
		return nil, fmt.Errorf("blockchain already exists")
	}

	// Validate miner wallet
	if minerWallet == nil {
		return nil, fmt.Errorf("miner wallet is required to create blockchain")
	}

	// Open database
	db, err := bbolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot open blockchain db: %v", err)
	}

	// Create PoS consensus and add the miner as initial validator
	posConsensus := consensus.NewPoSConsensus(db)
	err = posConsensus.AddStake(1000, minerWallet) // Initial stake for genesis validator
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to add genesis validator: %v", err)
	}

	var tip []byte
	err = db.Update(func(tx *bbolt.Tx) error {
		// Create coinbase transaction with miner's address
		cbtx := transaction.NewCoinbaseTx(minerWallet.PublicKey, genesisCoinbaseData)

		// Use PoS to propose the genesis block
		genesisBlock, err := posConsensus.ProposeBlock(minerWallet, []*transaction.Transaction{cbtx}, []byte{}, []byte{})
		if err != nil {
			return fmt.Errorf("failed to propose genesis block: %v", err)
		}

		// Create blocks bucket
		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			return err
		}

		// Store the genesis block
		blockData, err := genesisBlock.Serialize()
		if err != nil {
			return err
		}

		err = b.Put(genesisBlock.Hash, blockData)
		if err != nil {
			return err
		}

		// Store the last block hash
		err = b.Put([]byte(lastHashKey), genesisBlock.Hash)
		if err != nil {
			return err
		}

		tip = genesisBlock.Hash
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create genesis block: %v", err)
	}

	// Create blockchain instance with PoS consensus
	bc := Blockchain{tip, db, posConsensus, sync.RWMutex{}}

	// Initialize UTXO set
	utxo := UTXOSet{&bc}
	err = utxo.Reindex()
	if err != nil {
		bc.CloseDB()
		return nil, fmt.Errorf("failed to initialize UTXO set: %v", err)
	}

	return &bc, nil
}

// MineBlock creates a new block using PoS consensus (validator proposing)
func (bc *Blockchain) MineBlock(transactions []*transaction.Transaction, proposerWallet *wallet.Wallet) (*block.Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for _, tx := range transactions {
		if !tx.IsCoinbase() {
			if err := bc.VerifyTransaction(tx); err != nil {
				return nil, fmt.Errorf("invalid transaction: %v", err)
			}
		}
	}

	lastHash := bc.tip

	// Use PoS consensus to propose the block
	newBlock, err := bc.consensus.ProposeBlock(proposerWallet, transactions, lastHash, bc.tip)
	if err != nil {
		return nil, fmt.Errorf("failed to propose block with PoS: %v", err)
	}

	// Validate the proposed block
	prevTXs := make(map[string]transaction.Transaction)
	for _, tx := range transactions {
		if !tx.IsCoinbase() {
			for _, vin := range tx.Vin {
				prevTX, err := bc.FindTransaction(vin.Txid)
				if err != nil {
					return nil, err
				}
				prevTXs[hex.EncodeToString(prevTX.ID)] = *prevTX
			}
		}
	}

	valid, err := bc.consensus.ValidateBlock(newBlock, prevTXs)
	if err != nil || !valid {
		return nil, fmt.Errorf("block validation failed: %v", err)
	}

	// Store the validated block
	err = bc.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockData, err := newBlock.Serialize()
		if err != nil {
			return err
		}

		err = b.Put(newBlock.Hash, blockData)
		if err != nil {
			return err
		}

		err = b.Put([]byte(lastHashKey), newBlock.Hash)
		if err != nil {
			return err
		}

		bc.tip = newBlock.Hash
		return nil
	})

	return newBlock, err
}

// getAdjustedTargetBits calculates and returns the current target Bits for mining
// it adjusts difficulty every DIFFICULTY_ADJUSTMENT_BLOCKS blocks
func (bc *Blockchain) getAdjustedTargetBits() (int64, error) {
	var currentBlock *block.Block
	var err error
	// Get the current tip block to determine current height
	currentHash := bc.tip
	err = bc.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockData := b.Get(currentHash)
		if blockData == nil {
			return fmt.Errorf("tip block not found")
		}
		currentBlock, err = block.DeserializeBlock(blockData)
		return err
	})
	if err != nil {
		return 0, err
	}
	// For the genesis block
	if currentBlock.IsGenesisBlock() {
		return INITIAL_TARGET_BITS, nil
	}
	// Note: prevBlock not needed for current implementation
	// but keeping this comment for future reference if needed

	// Get the current tip block height: Then i will count back to get the height for adjustment calculation.
	currentHeight := 0
	bci := bc.Iterator()
	for {
		b, err := bci.Next()
		if err != nil {
			return 0, err
		}
		if b == nil {
			break
		}
		currentHeight++
		if bytes.Equal(b.Hash, currentBlock.Hash) {
			break
		}
	}

	if currentHeight%DIFFICULTY_ADJUSTMENT_BLOCKS == 0 && currentHeight != 0 {
		// find the first block of the last adjustment period
		firstBlockOfPeriodHash := currentBlock.Hash
		iter := bc.Iterator()
		for i := 0; i < DIFFICULTY_ADJUSTMENT_BLOCKS; i++ {
			b, err := iter.Next()
			if err != nil || b == nil {
				return 0, fmt.Errorf("failed to get block for difficulty calculation: %v", err)
			}
			firstBlockOfPeriodHash = b.Hash
		}
		firstBlockOfPeriod, err := bc.FindBlock(firstBlockOfPeriodHash)
		if err != nil {
			return 0, fmt.Errorf("failed to get block for difficulty calculation: %v", err)
		}

		actualTimeTaken := currentBlock.Timestamp - firstBlockOfPeriod.Timestamp
		expectedTimeTaken := int64(DIFFICULTY_ADJUSTMENT_BLOCKS) * TARGET_BLOCK_TIME_SECONDS

		// Get the current target (from the previous block)
		// For now, we'll use a standard calculation based on target bits
		// You may need to modify this based on your block structure
		prevTargetBits := INITIAL_TARGET_BITS // Default to initial if no stored target bits
		if currentHeight > DIFFICULTY_ADJUSTMENT_BLOCKS {
			// Try to get target bits from previous adjustment period
			// This is a simplified approach - ideally store target bits in block
			prevTargetBits = INITIAL_TARGET_BITS
		}

		// Calculate current target from target bits
		currentTarget := big.NewInt(1)
		currentTarget.Lsh(currentTarget, uint(256-prevTargetBits))

		// Calculate new target
		newTarget := new(big.Int).Set(currentTarget)
		newTarget.Mul(newTarget, big.NewInt(actualTimeTaken))
		newTarget.Div(newTarget, big.NewInt(expectedTimeTaken))

		// Apply limits to prevent extreme difficulty changes
		maxTarget := new(big.Int).Set(currentTarget)
		maxTarget.Mul(maxTarget, big.NewInt(MAX_ADJUSTMENT_FACTOR))

		minTarget := new(big.Int).Set(currentTarget)
		minTarget.Div(minTarget, big.NewInt(MAX_ADJUSTMENT_FACTOR))

		if newTarget.Cmp(maxTarget) == 1 { // if newTarget > maxTarget
			newTarget.Set(maxTarget)
		} else if newTarget.Cmp(minTarget) == -1 { // if newTarget < minTarget
			newTarget.Set(minTarget)
		}

		// Convert new target back to bits
		newTargetBits := 256 - newTarget.BitLen()
		if newTargetBits < 1 { // Ensure targetBits doesn't go below 1
			newTargetBits = 1
		}
		if newTargetBits > 255 { // Ensure targetBits doesn't go above 255
			newTargetBits = 255
		}

		return int64(newTargetBits), nil

	} else {
		// If not adjustment period, use the targetBits from the previous block
		// This is a simplified approach - ideally you'd store target bits in the block
		// For now, we'll use the initial target bits as a fallback
		return INITIAL_TARGET_BITS, nil
	}
}

// FindBlock finds  block by its hash (new helper func)
func (bc *Blockchain) FindBlock(hash []byte) (*block.Block, error) {
	var blockData []byte
	err := bc.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockData = b.Get(hash)
		if blockData == nil {
			return fmt.Errorf("block not found for hash: %x", hash)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	blk, err := block.DeserializeBlock(blockData)
	if err != nil {
		return nil, err
	}
	return blk, nil
}

// Iterator returns a BlockchainIterator
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return &BlockchainIterator{bc.tip, bc.db}
}

// Next returns the next block from the iterator
func (i *BlockchainIterator) Next() (*block.Block, error) {
	var blockData []byte

	err := i.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockData = b.Get(i.currentHash)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if blockData == nil {
		return nil, nil
	}

	block, err := block.DeserializeBlock(blockData)
	if err != nil {
		return nil, err
	}

	i.currentHash = block.PrevBlockHash
	return block, nil
}

// FindUTXO finds and returns all unspent transaction outputs
func (bc *Blockchain) FindUTXO() map[string][]transaction.TxOutput {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	UTXO := make(map[string][]transaction.TxOutput)
	spentTXOs := make(map[string][]int)
	// Don't call Iterator() as it tries to acquire the same lock
	bci := &BlockchainIterator{bc.tip, bc.db}

	for {
		block, err := bci.Next()
		if err != nil {
			break
		}
		if block == nil {
			break
		}

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Vout {
				// Was the output spent?
				if spentTXOs[txID] != nil {
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx {
							continue Outputs
						}
					}
				}

				outs := UTXO[txID]
				outs = append(outs, out)
				UTXO[txID] = outs
			}

			if !tx.IsCoinbase() {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return UTXO
}

// SignTransaction signs a transaction using the provided wallet
func (bc *Blockchain) SignTransaction(tx *transaction.Transaction, w *wallet.Wallet) error {
	if tx.IsCoinbase() {
		return nil
	}

	prevTXs := make(map[string]transaction.Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return err
		}
		if prevTX == nil {
			return fmt.Errorf("referenced transaction not found: %x", vin.Txid)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = *prevTX
	}

	return tx.Sign(w, prevTXs)
}

// VerifyTransaction verifies transaction input signatures
func (bc *Blockchain) VerifyTransaction(tx *transaction.Transaction) error {
	if tx.IsCoinbase() {
		return nil
	}

	prevTXs := make(map[string]transaction.Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return err
		}
		if prevTX == nil {
			return fmt.Errorf("referenced transaction not found: %x", vin.Txid)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = *prevTX
	}

	valid, err := tx.Verify(prevTXs)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid transaction signature")
	}

	return nil
}

// FindTransaction finds a transaction by its ID
func (bc *Blockchain) FindTransaction(ID []byte) (*transaction.Transaction, error) {
	// Don't call Iterator() as it tries to acquire the same lock
	// Instead create iterator manually
	bci := &BlockchainIterator{bc.tip, bc.db}

	for {
		block, err := bci.Next()
		if err != nil {
			return nil, err
		}
		if block == nil {
			break
		}

		for _, tx := range block.Transactions {
			if bytes.Equal(tx.ID, ID) {
				return tx, nil
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return nil, fmt.Errorf("transaction not found")
}

// GetConsensus returns the consensus mechanism used by the blockchain
func (bc *Blockchain) GetConsensus() consensus.Consensus {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.consensus
}

// CompactDB compacts the BoltDB file to reduce fragmentation and file size.
// This function should be used periodically to optimize database performance.
// 
// The compaction process:
// 1. Closes the current database connection
// 2. Opens the database for compaction
// 3. Creates a temporary compacted database file
// 4. Copies all data from the original to the compacted file
// 5. Replaces the original file with the compacted version
// 6. Reopens the database connection
//
// Note: This operation may take some time for large databases and requires
// sufficient disk space for the temporary file.
func (bc *Blockchain) CompactDB() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Close the existing DB connection before compaction
	if err := bc.db.Close(); err != nil {
		return fmt.Errorf("failed to close DB before compaction: %v", err)
	}

	// Open the database in read-write mode for compaction
	db, err := bbolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return fmt.Errorf("cannot open blockchain db for compaction: %v", err)
	}
	defer db.Close() // Ensure the temporary DB connection is closed

	fmt.Println("Starting database compaction...")
	// BoltDB's Compact method requires an output path for the compacted DB
	// A common pattern is to compact to a temporary file, then replace the original
	tempDbFile := dbFile + ".tmp"
	tempDb, err := bbolt.Open(tempDbFile, 0600, nil)
	if err != nil {
		return fmt.Errorf("cannot create temporary db file for compaction: %v", err)
	}
	defer os.Remove(tempDbFile) // Clean up temp file on exit
	defer tempDb.Close()

	err = bbolt.Compact(tempDb, db, 0) // Compact from original DB to temporary DB
	if err != nil {
		return fmt.Errorf("failed to compact database: %v", err)
	}

	// Close both database connections before file operations
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close source database: %v", err)
	}
	if err := tempDb.Close(); err != nil {
		return fmt.Errorf("failed to close temporary database: %v", err)
	}

	// Replace the original database file with the compacted one
	if err := os.Rename(tempDbFile, dbFile); err != nil {
		return fmt.Errorf("failed to replace original db with compacted db: %v", err)
	}

	fmt.Println("Database compaction complete.")

	// Reopen the main DB connection for the Blockchain instance
	db, err = bbolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return fmt.Errorf("cannot reopen blockchain db after compaction: %v", err)
	}
	bc.db = db // Update the Blockchain instance's DB connection
	return nil
}

// CloseDB closes the database
func (bc *Blockchain) CloseDB() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.db.Close()
}

// DbExists checks if the blockchain database exists
func DbExists() bool {
	_, err := os.Stat(dbFile)
	return !os.IsNotExist(err)
}
