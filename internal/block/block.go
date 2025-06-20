package block

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/OmSingh2003/decentralized-ledger/internal/transaction"
	"github.com/OmSingh2003/decentralized-ledger/pkg/serialization"
)

// Block represents a block in the blockchain
type Block struct {
	Timestamp       int64                      // Records when block was created/mined
	Transactions    []*transaction.Transaction // stores Transactions
	PrevBlockHash   []byte                     // Stores the Hash of previous Block in the chain
	Hash            []byte                     // Stores the Hash of current block in the chain
	Nonce           int                        // Number used in proof of work (retained for structural consistency, might be zero in PoS)
	Bits            int64                      // Stores the difficulty target bits for this block (retained, might be zero or repurposed in PoS)
	ValidatorPubKey []byte                     // Public key of the validator who signed this block
	Signature       []byte                     // Signature of the block by the validator
	mu              sync.RWMutex               // Mutex for thread safety
}

// NewBlock creates and returns a new Block
// In PoS, the hash, nonce, bits, validator key, and signature are filled later by the consensus mechanism.
func NewBlock(transactions []*transaction.Transaction, prevBlockHash []byte) *Block {
	block := &Block{
		Timestamp:       time.Now().Unix(),
		Transactions:    transactions,
		PrevBlockHash:   prevBlockHash,
		Hash:            []byte{},
		Nonce:           0,
		Bits:            0,
		ValidatorPubKey: nil, // Initialize new fields
		Signature:       nil, // Initialize new fields
	}
	// The actual Hash, Nonce, Bits, ValidatorPubKey, and Signature will be set by the consensus mechanism (PoW or PoS)
	return block
}

// IsGenesisBlock checks if this block is a genesis block
func (b *Block) IsGenesisBlock() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.PrevBlockHash) == 0
}

// HashTransactions returns a hash of the transactions in the block
// This is a thread-safe public method
func (b *Block) HashTransactions() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.hashTransactionsInternal()
}

// hashTransactionsInternal is an internal method that doesn't use locks
// It should only be called when the lock is already held or when thread safety isn't required
func (b *Block) hashTransactionsInternal() []byte {
	var txHashes [][]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}
	txHash := sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}

// PrepareData prepares data for hashing for PoW (still used by PoWConsensus)
// For PoS, a similar function might be needed that includes PoS-specific header fields.
func (b *Block) PrepareData(nonce int, targetBits int64) []byte {
	data := bytes.Join(
		[][]byte{
			b.PrevBlockHash,
			b.hashTransactionsInternal(),
			IntToHex(b.Timestamp),
			IntToHex(targetBits),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)
	return data
}

// GetHashableDataPoS prepares data for hashing specifically for PoS block signature.
// This includes all relevant fields that define the block's identity before signing.
func (b *Block) GetHashableDataPoS() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	data := bytes.Join(
		[][]byte{
			b.PrevBlockHash,
			b.hashTransactionsInternal(), // Merkle root/hash of transactions
			IntToHex(b.Timestamp),
			IntToHex(b.Bits),         // Might be 0 or repurposed in PoS
			IntToHex(int64(b.Nonce)), // Might be 0 or repurposed in PoS
			// b.ValidatorPubKey should be included here if it's set before signing
			// If ValidatorPubKey is set after signing, it shouldn't be included.
			b.ValidatorPubKey,
		},
		[]byte{},
	)
	return data
}

// Serialize serializes the block
func (b *Block) Serialize() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Convert transactions to serializable format
	var serializableTransactions []serialization.SerializableTransaction
	for _, tx := range b.Transactions {
		serTx := serialization.SerializableTransaction{
			ID: tx.ID,
		}
		
		// Convert inputs
		for _, input := range tx.Vin {
			serTx.Vin = append(serTx.Vin, serialization.SerializableTxInput{
				Txid:      input.Txid,
				Vout:      input.Vout,
				Signature: input.Signature,
				PubKey:    input.PubKey,
			})
		}
		
		// Convert outputs
		for _, output := range tx.Vout {
			serTx.Vout = append(serTx.Vout, serialization.SerializableTxOutput{
				Value:      output.Value,
				PubKeyHash: output.PubKeyHash,
			})
		}
		
		serializableTransactions = append(serializableTransactions, serTx)
	}

	data := serialization.SerializeBlock(
		b.Timestamp,
		serializableTransactions,
		b.PrevBlockHash,
		b.Hash,
		b.Nonce,
		b.Bits,
		b.ValidatorPubKey,
		b.Signature,
	)

	if data == nil {
		return nil, fmt.Errorf("failed to serialize block")
	}

	return data, nil
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) (*Block, error) {
	blockData, err := serialization.DeserializeBlock(d)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %v", err)
	}
	
	// Convert back to Block struct
	block := &Block{
		Timestamp:       blockData.Timestamp,
		PrevBlockHash:   blockData.PrevBlockHash,
		Hash:            blockData.Hash,
		Nonce:           blockData.Nonce,
		Bits:            blockData.Bits,
		ValidatorPubKey: blockData.ValidatorPubKey,
		Signature:       blockData.Signature,
		mu:              sync.RWMutex{}, // Initialize the mutex
	}
	
	// Convert transactions
	for _, serTx := range blockData.Transactions {
		tx := &transaction.Transaction{
			ID: serTx.ID,
		}
		
		// Convert inputs
		for _, serInput := range serTx.Vin {
			tx.Vin = append(tx.Vin, transaction.TxInput{
				Txid:      serInput.Txid,
				Vout:      serInput.Vout,
				Signature: serInput.Signature,
				PubKey:    serInput.PubKey,
			})
		}
		
		// Convert outputs
		for _, serOutput := range serTx.Vout {
			tx.Vout = append(tx.Vout, transaction.TxOutput{
				Value:      serOutput.Value,
				PubKeyHash: serOutput.PubKeyHash,
			})
		}
		
		block.Transactions = append(block.Transactions, tx)
	}
	
	return block, nil
}

// IntToHex converts an int64 to a byte array
func IntToHex(num int64) []byte {
	return []byte(strconv.FormatInt(num, 10))
}

// ValidateBlock validates the block and its transactions in parallel
func (b *Block) ValidateBlock(prevTXs map[string]transaction.Transaction) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Special case for genesis block
	if len(b.PrevBlockHash) == 0 {
		if len(b.Transactions) != 1 || !b.Transactions[0].IsCoinbase() {
			return fmt.Errorf("invalid genesis block: must have exactly one coinbase transaction")
		}
		return nil
	}

	// Regular block validation common to both PoW/PoS
	if len(b.Transactions) == 0 {
		return fmt.Errorf("block must contain at least one transaction")
	}

	// Ensure first transaction is coinbase
	if !b.Transactions[0].IsCoinbase() {
		return fmt.Errorf("first transaction must be coinbase")
	}

	// Validate transactions in parallel
	var wg sync.WaitGroup
	errs := make(chan error, len(b.Transactions))

	for i, tx := range b.Transactions {
		if tx.IsCoinbase() {
			continue
		}

		wg.Add(1)
		go func(tx *transaction.Transaction, i int) {
			defer wg.Done()
			if err := tx.ValidateTransaction(prevTXs); err != nil {
				errs <- fmt.Errorf("invalid transaction at index %d: %v", i, err)
			}
		}(tx, i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		// Return the first error found
		return err
	}

	return nil
}

// UpdateHash updates the block's hash based on its current state (used by PoW)
func (b *Block) UpdateHash() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Use Bits from the block itself for PoW hash
	data := b.PrepareData(b.Nonce, b.Bits)
	hash := sha256.Sum256(data)
	b.Hash = hash[:]
	return nil
}

// CalculateHash calculates and returns the hash of the block (used by PoW)
func (b *Block) CalculateHash() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Use Bits from the block itself for PoW hash
	data := b.PrepareData(b.Nonce, b.Bits)
	hash := sha256.Sum256(data)
	return hash[:]
}

// GetPoSHash calculates and returns the hash of the block specifically for PoS
// This hash should be used for signing and for the block's final ID.
func (b *Block) GetPoSHash() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Hash the data relevant to the block's identity for PoS
	data := b.GetHashableDataPoS()
	hash := sha256.Sum256(data)
	return hash[:]
}

// SetNonce sets the nonce of the block in a thread-safe manner
func (b *Block) SetNonce(nonce int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Nonce = nonce
}

// GetHash returns the hash of the block in a thread-safe manner
func (b *Block) GetHash() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Hash
}

// GetNonce returns the nonce of the block in a thread-safe manner
func (b *Block) GetNonce() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Nonce
}

// SetBits sets the bits of the block in a thread-safe manner
func (b *Block) SetBits(bits int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Bits = bits
}

// GetBits returns the bits of the block in a thread-safe manner
func (b *Block) GetBits() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Bits
}

// SetValidatorPubKey sets the validator's public key
func (b *Block) SetValidatorPubKey(pubKey []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ValidatorPubKey = pubKey
}

// GetValidatorPubKey returns the validator's public key
func (b *Block) GetValidatorPubKey() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ValidatorPubKey
}

// SetSignature sets the block's signature
func (b *Block) SetSignature(sig []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Signature = sig
}

// GetSignature returns the block's signature
func (b *Block) GetSignature() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Signature
}
