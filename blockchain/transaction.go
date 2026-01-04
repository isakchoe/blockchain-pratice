package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
)

type Transaction struct {
	ID   []byte
	Ins  []TxInput
	Outs []TxOutput
}

type TxInput struct {
	TxID     []byte
	OutIndex int
	FromAddr string
}

type TxOutput struct {
	Value  int
	PubKey string
}

func (t *Transaction) SetID() {
	var encoded bytes.Buffer
	gob.NewEncoder(&encoded).Encode(t)
	hash := sha256.Sum256(encoded.Bytes())
	t.ID = hash[:]
}

func (t *Transaction) IsCoinbase() bool {
	return len(t.Ins) == 1 && len(t.Ins[0].TxID) == 0 && t.Ins[0].OutIndex == -1
}

func NewCoinbaseTX(to, data string) *Transaction {
	txin := TxInput{[]byte{}, -1, data}
	txout := TxOutput{50, to}
	tx := Transaction{nil, []TxInput{txin}, []TxOutput{txout}}
	tx.SetID()
	return &tx
}
