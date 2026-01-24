package blockchain

import (
	"blockchain/wallet"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
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
	PubKey    []byte // 지갑의 원본 공개키 (64바이트)
}

type TxOutput struct {
	Value      int
	PubKeyHash []byte // 주소 대신 20바이트 해시 저장 (자물쇠)
}

// 특정 주소의 주인인지 확인하는 헬퍼 함수
func (out *TxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

// 새로운 Output 생성 시 주소를 해시로 변환하여 저장
func NewTxOutput(value int, address string) *TxOutput {
	out := &TxOutput{value, nil}
	out.PubKeyHash = wallet.Base58ToPubKeyHash(address)
	return out
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

// ⭐️ 수정됨: Coinbase 출력도 PubKeyHash를 사용해야 함
func NewCoinbaseTX(to, data string) *Transaction {
	txin := TxInput{[]byte{}, -1, nil, []byte(data)}

	// 보상금 출력 시 주소를 해시로 변환
	pubKeyHash := wallet.Base58ToPubKeyHash(to)
	txout := TxOutput{50, pubKeyHash}

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
		// ⭐️ 수정: 서명할 때 참조하는 아웃풋의 자물쇠(PubKeyHash)를 데이터로 넣음
		txCopy.Ins[inID].PubKey = prevTx.Outs[vin.OutIndex].PubKeyHash
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
		prevOut := prevTx.Outs[vin.OutIndex]

		// ⭐️ [보안 검증 1] 소유권 확인
		// 내가 제출한 공개키의 해시가, 이전 주인이 걸어둔 자물쇠(PubKeyHash)와 일치하는가?
		if !bytes.Equal(wallet.HashPubKey(vin.PubKey), prevOut.PubKeyHash) {
			fmt.Printf("🚨 보안 경고: 남의 돈을 쓰려는 시도가 감지됨! (ID: %x)\n", tx.ID)
			return false
		}

		// 서명 검증을 위한 데이터 준비 (Sign 시와 동일한 로직)
		txCopy.Ins[inID].Signature = nil
		txCopy.Ins[inID].PubKey = prevOut.PubKeyHash
		txCopy.ID = txCopy.Hash()
		txCopy.Ins[inID].PubKey = nil

		// ⭐️ [보안 검증 2] 서명 확인
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
			fmt.Printf("🚨 보안 경고: 서명이 유효하지 않음!\n")
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
