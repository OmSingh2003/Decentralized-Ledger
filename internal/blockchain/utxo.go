package blockchain

import (
	"bytes"
	"encoding/hex"
	"log"

	"github.com/OmSingh2003/decentralized-ledger/internal/block"
	"github.com/OmSingh2003/decentralized-ledger/internal/transaction"
	"go.etcd.io/bbolt"
)

const utxoBucket = "chainstate"

// UTXOSet represents UTXO set
type UTXOSet struct {
	Blockchain *Blockchain
}

// Reindex rebuilds the UTXO set
// In internal/blockchain/utxo.go

func (u UTXOSet) Reindex() error {
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)
	UTXO := u.Blockchain.FindUTXO()

	// Combined database update
	err := db.Update(func(tx *bbolt.Tx) error {
		// Delete the old bucket
		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}

		// Create a new bucket
		b, err := tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		// Add all UTXOs to the new bucket
		for txID, outs := range UTXO {
			key, err := hex.DecodeString(txID)
			if err != nil {
				return err
			}
			err = b.Put(key, transaction.SerializeOutputs(outs))
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// FindSpendableOutputs finds and returns unspent outputs to reference in inputs
func (u UTXOSet) FindSpendableOutputs(pubkeyHash []byte, amount int) (int, map[string][]int, error) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.db

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		if b == nil {
			return bbolt.ErrBucketNotFound
		}

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			txID := hex.EncodeToString(k)
			outs := transaction.DeserializeOutputs(v)

			for outIdx, out := range outs {
				if out.IsLockedWithKey(pubkeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

					if accumulated >= amount {
						break
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return 0, nil, err
	}

	return accumulated, unspentOutputs, nil
}

// FindUTXO finds UTXO for a public key hash
func (u UTXOSet) FindUTXO(pubKeyHash []byte) []transaction.TxOutput {
	var UTXOs []transaction.TxOutput
	db := u.Blockchain.db

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		if b == nil {
			return bbolt.ErrBucketNotFound
		}

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs := transaction.DeserializeOutputs(v)

			for _, out := range outs {
				// Check if this is a query for all UTXOs or specifically for this pubKeyHash
				if pubKeyHash == nil {
					UTXOs = append(UTXOs, out)
				} else if bytes.Compare(out.PubKeyHash, pubKeyHash) == 0 {
					UTXOs = append(UTXOs, out)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Printf("Error finding UTXO: %v", err)
		return nil
	}

	return UTXOs
}

// Update updates the UTXO set with the transactions from the Block
func (u UTXOSet) Update(block *block.Block) error {
	db := u.Blockchain.db

	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		if b == nil {
			return bbolt.ErrBucketNotFound
		}

		for _, tx := range block.Transactions {
			if !tx.IsCoinbase() {
				for _, vin := range tx.Vin {
					updatedOuts := []transaction.TxOutput{}
					outsBytes := b.Get(vin.Txid)
					outs := transaction.DeserializeOutputs(outsBytes)

					for outIdx, out := range outs {
						if outIdx != vin.Vout {
							updatedOuts = append(updatedOuts, out)
						}
					}

					if len(updatedOuts) == 0 {
						err := b.Delete(vin.Txid)
						if err != nil {
							return err
						}
					} else {
						err := b.Put(vin.Txid, transaction.SerializeOutputs(updatedOuts))
						if err != nil {
							return err
						}
					}
				}
			}

			newOutputs := transaction.SerializeOutputs(tx.Vout)
			err := b.Put(tx.ID, newOutputs)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}
