package transaction

import (
    "bytes"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "log"
    
    "github.com/OmSingh2003/decentralized-ledger/internal/wallet"
    "github.com/OmSingh2003/decentralized-ledger/pkg/serialization"
)

// Rest of the file content remains the same...

// Transaction represents a blockchain transaction
type Transaction struct {
    ID   []byte
    Vin  []TxInput
    Vout []TxOutput
}

// TxInput represents a transaction input
type TxInput struct {
    Txid      []byte // The ID of the transaction containing the output to spend
    Vout      int    // The index of the output in the transaction
    Signature []byte // The digital signature that proves ownership
    PubKey    []byte // The public key of the sender
}

// TxOutput represents a transaction output
type TxOutput struct {
    Value      int    // The amount of coins
    PubKeyHash []byte // The hash of the public key (address) of the recipient
}

// Hash returns the hash of the Transaction
func (tx *Transaction) Hash() []byte {
    var hash [32]byte
    txCopy := *tx
    txCopy.ID = []byte{}
    
    encoded, err := serializeTransaction(txCopy)
    if err != nil {
        log.Panic(err)
    }
    
    hash = sha256.Sum256(encoded)
    return hash[:]
}

// serializeTransaction serializes a transaction
func serializeTransaction(tx Transaction) ([]byte, error) {
    // Convert to serializable format
    var serVin []serialization.SerializableTxInput
    for _, input := range tx.Vin {
        serVin = append(serVin, serialization.SerializableTxInput{
            Txid:      input.Txid,
            Vout:      input.Vout,
            Signature: input.Signature,
            PubKey:    input.PubKey,
        })
    }
    
    var serVout []serialization.SerializableTxOutput
    for _, output := range tx.Vout {
        serVout = append(serVout, serialization.SerializableTxOutput{
            Value:      output.Value,
            PubKeyHash: output.PubKeyHash,
        })
    }
    
    data := serialization.SerializeTransaction(tx.ID, serVin, serVout)
    if data == nil {
        return nil, fmt.Errorf("failed to encode transaction")
    }
    
    return data, nil
}

// IsCoinbase checks whether the transaction is coinbase
func (tx *Transaction) IsCoinbase() bool {
    return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

// TrimmedCopy creates a trimmed copy of Transaction to be used in signing
func (tx *Transaction) TrimmedCopy() Transaction {
    var inputs []TxInput
    var outputs []TxOutput

    for _, vin := range tx.Vin {
        inputs = append(inputs, TxInput{vin.Txid, vin.Vout, nil, nil})
    }

    for _, vout := range tx.Vout {
        outputs = append(outputs, TxOutput{vout.Value, vout.PubKeyHash})
    }

    txCopy := Transaction{tx.ID, inputs, outputs}
    return txCopy
}

// Sign signs each input of a Transaction
func (tx *Transaction) Sign(walletInstance *wallet.Wallet, prevTXs map[string]Transaction) error {
    if tx.IsCoinbase() {
        return nil
    }

    // Verify all referenced inputs exist in prevTXs
    for _, vin := range tx.Vin {
        if len(vin.Txid) == 0 {
            continue // Skip coinbase
        }
        
        txID := hex.EncodeToString(vin.Txid)
        _, exists := prevTXs[txID]
        if !exists {
            return fmt.Errorf("referenced input transaction not found: %s", txID)
        }
    }

    txCopy := tx.TrimmedCopy()

    for inID, vin := range txCopy.Vin {
        if len(vin.Txid) == 0 {
            continue // Skip coinbase
        }
        
        txID := hex.EncodeToString(vin.Txid)
        prevTx := prevTXs[txID]
        txCopy.Vin[inID].Signature = nil
        txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash
        txCopy.ID = txCopy.Hash()
        txCopy.Vin[inID].PubKey = nil

        // Use wallet's SignData function for signing
        signature, err := walletInstance.SignData(txCopy.ID)
        if err != nil {
            return fmt.Errorf("failed to sign transaction input: %v", err)
        }
        
        tx.Vin[inID].Signature = signature
    }

    return nil
}

// Verify verifies signatures of Transaction inputs
func (tx *Transaction) Verify(prevTXs map[string]Transaction) (bool, error) {
    if tx.IsCoinbase() {
        return true, nil
    }

    // Verify all referenced inputs exist in prevTXs
    for _, vin := range tx.Vin {
        if len(vin.Txid) == 0 {
            continue // Skip coinbase
        }
        
        txID := hex.EncodeToString(vin.Txid)
        _, exists := prevTXs[txID]
        if !exists {
            return false, fmt.Errorf("referenced input transaction not found: %s", txID)
        }
    }

    txCopy := tx.TrimmedCopy()

    for inID, vin := range tx.Vin {
        if len(vin.Txid) == 0 {
            continue // Skip coinbase
        }
        
        txID := hex.EncodeToString(vin.Txid)
        prevTx := prevTXs[txID]
        txCopy.Vin[inID].Signature = nil
        txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash
        txCopy.ID = txCopy.Hash()
        txCopy.Vin[inID].PubKey = nil

        if !wallet.VerifySignature(vin.PubKey, txCopy.ID, vin.Signature) {
            return false, nil
        }
    }

    return true, nil
}

// ValidateTransaction validates a transaction
func (tx *Transaction) ValidateTransaction(prevTXs map[string]Transaction) error {
    if len(tx.ID) == 0 {
        return fmt.Errorf("transaction ID cannot be empty")
    }
    
    if len(tx.Vin) == 0 {
        return fmt.Errorf("transaction must have at least one input")
    }
    
    if len(tx.Vout) == 0 {
        return fmt.Errorf("transaction must have at least one output")
    }
    
    // Verify signatures if not a coinbase transaction
    if !tx.IsCoinbase() {
        valid, err := tx.Verify(prevTXs)
        if err != nil {
            return fmt.Errorf("signature verification error: %v", err)
        }
        if !valid {
            return fmt.Errorf("invalid transaction signature")
        }
    }
    
    return nil
}

// UsesKey checks whether the input uses the specified public key hash
func (in *TxInput) UsesKey(pubKeyHash []byte) bool {
    lockingHash := wallet.HashPubKey(in.PubKey)
    return bytes.Compare(lockingHash, pubKeyHash) == 0
}

// IsLockedWithKey checks if the output is locked with the specified public key hash
func (out *TxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
    return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}

// NewCoinbaseTx creates a new coinbase transaction
func NewCoinbaseTx(to []byte, data string) *Transaction {
    if data == "" {
        randData := make([]byte, 20)
        _, err := rand.Read(randData)
        if err != nil {
            log.Panic(err)
        }
        data = fmt.Sprintf("Reward to '%x'", randData)
    }

    txin := TxInput{
        Txid:      []byte{},
        Vout:      -1,
        Signature: nil,
        PubKey:    []byte(data),
    }

    // Ensure the pubKeyHash is derived properly from the public key
    pubKeyHash := wallet.HashPubKey(to)

    txout := TxOutput{
        Value:      50, // Mining reward
        PubKeyHash: pubKeyHash,
    }

    tx := &Transaction{
        ID:   []byte{},
        Vin:  []TxInput{txin},
        Vout: []TxOutput{txout},
    }

    tx.ID = tx.Hash()

    return tx
}

// NewUTXOTransaction creates a new transaction
func NewUTXOTransaction(w *wallet.Wallet, to []byte, amount int, findSpendableOutputs func([]byte, int) (int, map[string][]int, error)) (*Transaction, error) {
    var inputs []TxInput
    var outputs []TxOutput

    pubKeyHash := wallet.HashPubKey(w.PublicKey)

    acc, validOutputs, err := findSpendableOutputs(pubKeyHash, amount)
    if err != nil {
        return nil, fmt.Errorf("failed to find spendable outputs: %v", err)
    }

    if acc < amount {
        return nil, fmt.Errorf("not enough funds: got %d, need %d", acc, amount)
    }

    // Build a list of inputs
    for txid, outs := range validOutputs {
        txID, err := hex.DecodeString(txid)
        if err != nil {
            return nil, fmt.Errorf("failed to decode transaction ID: %v", err)
        }

        for _, out := range outs {
            input := TxInput{
                Txid:      txID,
                Vout:      out,
                Signature: nil,
                PubKey:    w.PublicKey,
            }
            inputs = append(inputs, input)
        }
    }

    // Create the outputs
    outputs = append(outputs, TxOutput{
        Value:      amount,
        PubKeyHash: to,
    })

    if acc > amount {
        outputs = append(outputs, TxOutput{
            Value:      acc - amount,
            PubKeyHash: pubKeyHash,
        })
    }

    tx := &Transaction{
        ID:   []byte{},
        Vin:  inputs,
        Vout: outputs,
    }
    
    tx.ID = tx.Hash()

    return tx, nil
}
