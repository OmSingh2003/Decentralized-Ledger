package serialization

import (
    blockpb "github.com/OmSingh2003/decentralized-ledger/proto/block"
    transactionpb "github.com/OmSingh2003/decentralized-ledger/proto/transaction"
    "google.golang.org/protobuf/proto"
)

// SerializeBlock serializes a Block struct to protobuf bytes
func SerializeBlock(timestamp int64, transactions []SerializableTransaction, prevBlockHash []byte, hash []byte, nonce int, bits int64, validatorPubKey []byte, signature []byte) []byte {
    pbBlock := &blockpb.Block{
        Timestamp:       timestamp,
        PrevBlockHash:   prevBlockHash,
        Hash:            hash,
        Nonce:           int32(nonce),
        Bits:            bits,
        ValidatorPubKey: validatorPubKey,
        Signature:       signature,
    }

    // Convert transactions
    for _, tx := range transactions {
        pbTx := &transactionpb.Transaction{
            Id: tx.ID,
        }
        
        // Convert inputs
        for _, input := range tx.Vin {
            pbTx.Vin = append(pbTx.Vin, &transactionpb.TxInput{
                Txid:      input.Txid,
                Vout:      int32(input.Vout),
                Signature: input.Signature,
                PubKey:    input.PubKey,
            })
        }
        
        // Convert outputs
        for _, output := range tx.Vout {
            pbTx.Vout = append(pbTx.Vout, &transactionpb.TxOutput{
                Value:      int32(output.Value),
                PubKeyHash: output.PubKeyHash,
            })
        }
        
        pbBlock.Transactions = append(pbBlock.Transactions, pbTx)
    }

    data, err := proto.Marshal(pbBlock)
    if err != nil {
        return nil
    }
    return data
}

// DeserializeBlock deserializes protobuf bytes to block data
func DeserializeBlock(d []byte) (*BlockData, error) {
    pbBlock := &blockpb.Block{}
    err := proto.Unmarshal(d, pbBlock)
    if err != nil {
        return nil, err
    }
    
    blockData := &BlockData{
        Timestamp:       pbBlock.Timestamp,
        PrevBlockHash:   pbBlock.PrevBlockHash,
        Hash:            pbBlock.Hash,
        Nonce:           int(pbBlock.Nonce),
        Bits:            pbBlock.Bits,
        ValidatorPubKey: pbBlock.ValidatorPubKey,
        Signature:       pbBlock.Signature,
    }
    
    // Convert transactions
    for _, pbTx := range pbBlock.Transactions {
        tx := SerializableTransaction{
            ID: pbTx.Id,
        }
        
        // Convert inputs
        for _, pbInput := range pbTx.Vin {
            tx.Vin = append(tx.Vin, SerializableTxInput{
                Txid:      pbInput.Txid,
                Vout:      int(pbInput.Vout),
                Signature: pbInput.Signature,
                PubKey:    pbInput.PubKey,
            })
        }
        
        // Convert outputs
        for _, pbOutput := range pbTx.Vout {
            tx.Vout = append(tx.Vout, SerializableTxOutput{
                Value:      int(pbOutput.Value),
                PubKeyHash: pbOutput.PubKeyHash,
            })
        }
        
        blockData.Transactions = append(blockData.Transactions, tx)
    }
    
    return blockData, nil
}

// SerializableTransaction represents transaction data for serialization
type SerializableTransaction struct {
    ID   []byte
    Vin  []SerializableTxInput
    Vout []SerializableTxOutput
}

// SerializableTxInput represents transaction input data for serialization
type SerializableTxInput struct {
    Txid      []byte
    Vout      int
    Signature []byte
    PubKey    []byte
}

// SerializableTxOutput represents transaction output data for serialization
type SerializableTxOutput struct {
    Value      int
    PubKeyHash []byte
}

// BlockData represents block data for serialization
type BlockData struct {
    Timestamp       int64
    Transactions    []SerializableTransaction
    PrevBlockHash   []byte
    Hash            []byte
    Nonce           int
    Bits            int64
    ValidatorPubKey []byte
    Signature       []byte
}

// SerializeTransaction serializes a transaction to protobuf bytes
func SerializeTransaction(id []byte, vin []SerializableTxInput, vout []SerializableTxOutput) []byte {
    pbTx := &transactionpb.Transaction{
        Id: id,
    }
    
    // Convert inputs
    for _, input := range vin {
        pbTx.Vin = append(pbTx.Vin, &transactionpb.TxInput{
            Txid:      input.Txid,
            Vout:      int32(input.Vout),
            Signature: input.Signature,
            PubKey:    input.PubKey,
        })
    }
    
    // Convert outputs
    for _, output := range vout {
        pbTx.Vout = append(pbTx.Vout, &transactionpb.TxOutput{
            Value:      int32(output.Value),
            PubKeyHash: output.PubKeyHash,
        })
    }
    
    data, err := proto.Marshal(pbTx)
    if err != nil {
        return nil
    }
    return data
}

// SerializeOutputs serializes TxOutput array to protobuf bytes
func SerializeOutputs(outs []SerializableTxOutput) []byte {
    var pbOutputs []*transactionpb.TxOutput
    for _, out := range outs {
        pbOutputs = append(pbOutputs, &transactionpb.TxOutput{
            Value:      int32(out.Value),
            PubKeyHash: out.PubKeyHash,
        })
    }
    
    wrapper := &transactionpb.TxOutputList{
        Outputs: pbOutputs,
    }
    
    data, err := proto.Marshal(wrapper)
    if err != nil {
        return nil
    }
    
    return data
}

// DeserializeOutputs deserializes protobuf bytes to TxOutput array
func DeserializeOutputs(data []byte) ([]SerializableTxOutput, error) {
    wrapper := &transactionpb.TxOutputList{}
    err := proto.Unmarshal(data, wrapper)
    if err != nil {
        return nil, err
    }
    
    var outputs []SerializableTxOutput
    for _, pbOut := range wrapper.Outputs {
        outputs = append(outputs, SerializableTxOutput{
            Value:      int(pbOut.Value),
            PubKeyHash: pbOut.PubKeyHash,
        })
    }
    
    return outputs, nil
}
