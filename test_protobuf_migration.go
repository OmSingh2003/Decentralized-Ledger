package main

import (
	"fmt"
	"log"

	"github.com/OmSingh2003/decentralized-ledger/internal/block"
	"github.com/OmSingh2003/decentralized-ledger/internal/transaction"
	"github.com/OmSingh2003/decentralized-ledger/internal/wallet"
)

func main() {
	fmt.Println("Testing Protocol Buffer Migration...")
	
	// Create a test wallet
	w := wallet.NewWallet()
	pubKeyHash := wallet.HashPubKey(w.PublicKey)
	
	// Create a test coinbase transaction
	coinbaseTx := transaction.NewCoinbaseTx(pubKeyHash, "Genesis Block")
	
	// Create a test block
	testBlock := block.NewBlock([]*transaction.Transaction{coinbaseTx}, []byte{})
	testBlock.SetBits(24) // Set some test bits
	testBlock.SetNonce(12345) // Set some test nonce
	
	// Test serialization
	fmt.Println("Serializing block with Protocol Buffers...")
	serializedData, err := testBlock.Serialize()
	if err != nil {
		log.Fatal("Failed to serialize block:", err)
	}
	
	fmt.Printf("Serialized data size: %d bytes\n", len(serializedData))
	
	// Test deserialization
	fmt.Println("Deserializing block...")
	deserializedBlock, err := block.DeserializeBlock(serializedData)
	if err != nil {
		log.Fatal("Failed to deserialize block:", err)
	}
	
	// Verify the data
	fmt.Println("Verifying deserialized data...")
	if deserializedBlock.GetBits() != testBlock.GetBits() {
		log.Fatal("Bits mismatch!")
	}
	
	if deserializedBlock.GetNonce() != testBlock.GetNonce() {
		log.Fatal("Nonce mismatch!")
	}
	
	if len(deserializedBlock.Transactions) != len(testBlock.Transactions) {
		log.Fatal("Transaction count mismatch!")
	}
	
	// Test transaction output serialization
	fmt.Println("Testing transaction output serialization...")
	outputs := []transaction.TxOutput{
		{Value: 50, PubKeyHash: pubKeyHash},
		{Value: 25, PubKeyHash: pubKeyHash},
	}
	
	serializedOutputs := transaction.SerializeOutputs(outputs)
	deserializedOutputs := transaction.DeserializeOutputs(serializedOutputs)
	
	if len(deserializedOutputs) != len(outputs) {
		log.Fatal("Output count mismatch!")
	}
	
	for i, output := range outputs {
		if deserializedOutputs[i].Value != output.Value {
			log.Fatal("Output value mismatch!")
		}
	}
	
	fmt.Println("✅ Protocol Buffer migration test passed successfully!")
	fmt.Println("✅ All serialization and deserialization operations working correctly")
	fmt.Println("✅ Data integrity maintained during the migration")
}

