package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"math/big"
)

type Transaction struct {
	ID   []byte
	Ins  []TxInput
	Outs []TxOutput
}

type TxInput struct {
	TxID      []byte
	OutIndex  int
	Signature []byte
	PubKey    []byte // 지갑의 원본 공개키
}

type TxOutput struct {
	Value  int
	PubKey []byte // 수신자의 공개키(또는 주소)
}

func (tx *Transaction) Hash() []byte {
	var hash [32]byte
	txCopy := *tx
	txCopy.ID = []byte{}
	var encoded bytes.Buffer
	gob.NewEncoder(&encoded).Encode(txCopy)
	hash = sha256.Sum256(encoded.Bytes())
	return hash[:]
}

func (tx *Transaction) SetID() {
	tx.ID = tx.Hash()
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Ins) == 1 && len(tx.Ins[0].TxID) == 0 && tx.Ins[0].OutIndex == -1
}

func NewCoinbaseTX(to, data string) *Transaction {
	txin := TxInput{[]byte{}, -1, nil, []byte(data)}
	txout := TxOutput{50, []byte(to)}
	tx := Transaction{nil, []TxInput{txin}, []TxOutput{txout}}
	tx.SetID()
	return &tx
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}
	txCopy := tx.TrimmedCopy()

	for inID, vin := range txCopy.Ins {
		prevTx := prevTXs[hex.EncodeToString(vin.TxID)]
		txCopy.Ins[inID].Signature = nil
		txCopy.Ins[inID].PubKey = prevTx.Outs[vin.OutIndex].PubKey
		txCopy.ID = txCopy.Hash()
		txCopy.Ins[inID].PubKey = nil

		r, s, _ := ecdsa.Sign(rand.Reader, &privKey, txCopy.ID)
		tx.Ins[inID].Signature = append(r.Bytes(), s.Bytes()...)
	}
}

func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}
	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()

	for inID, vin := range tx.Ins {
		prevTx := prevTXs[hex.EncodeToString(vin.TxID)]
		txCopy.Ins[inID].Signature = nil
		txCopy.Ins[inID].PubKey = prevTx.Outs[vin.OutIndex].PubKey
		txCopy.ID = txCopy.Hash()
		txCopy.Ins[inID].PubKey = nil

		r, s := big.Int{}, big.Int{}
		sigLen := len(vin.Signature)
		r.SetBytes(vin.Signature[:(sigLen / 2)])
		s.SetBytes(vin.Signature[(sigLen / 2):])

		x, y := big.Int{}, big.Int{}
		keyLen := len(vin.PubKey)
		x.SetBytes(vin.PubKey[:(keyLen / 2)])
		y.SetBytes(vin.PubKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		if !ecdsa.Verify(&rawPubKey, txCopy.ID, &r, &s) {
			return false
		}
	}
	return true
}

func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TxInput
	for _, vin := range tx.Ins {
		inputs = append(inputs, TxInput{vin.TxID, vin.OutIndex, nil, nil})
	}
	return Transaction{tx.ID, inputs, tx.Outs}
}
