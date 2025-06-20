package transaction

import (
    "log"
    "github.com/OmSingh2003/decentralized-ledger/pkg/serialization"
)

// SerializeOutputs serializes TxOutput array
func SerializeOutputs(outs []TxOutput) []byte {
    // Convert to serializable format
    var serOutputs []serialization.SerializableTxOutput
    for _, out := range outs {
        serOutputs = append(serOutputs, serialization.SerializableTxOutput{
            Value:      out.Value,
            PubKeyHash: out.PubKeyHash,
        })
    }
    
    data := serialization.SerializeOutputs(serOutputs)
    if data == nil {
        log.Panic("failed to serialize outputs")
    }
    
    return data
}

// DeserializeOutputs deserializes TxOutput array
func DeserializeOutputs(data []byte) []TxOutput {
    serOutputs, err := serialization.DeserializeOutputs(data)
    if err != nil {
        log.Panic(err)
    }
    
    var outputs []TxOutput
    for _, serOut := range serOutputs {
        outputs = append(outputs, TxOutput{
            Value:      serOut.Value,
            PubKeyHash: serOut.PubKeyHash,
        })
    }
    
    return outputs
}
