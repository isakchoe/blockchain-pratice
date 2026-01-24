package blockchain

import (
	"blockchain/wallet"
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"github.com/boltdb/bolt"
)

const blocksBucket = "blocks"

type BlockChain struct {
	NewestHash string
	DB         *bolt.DB
}

var bInstance *BlockChain
var once sync.Once

func GetBlockchain(address string) *BlockChain {
	once.Do(func() {
		db, err := bolt.Open("blockchain.db", 0600, nil)
		if err != nil {
			log.Fatal(err)
		}

		var newestHash string
		err = db.Update(func(tx *bolt.Tx) error {
			bucket, _ := tx.CreateBucketIfNotExists([]byte(blocksBucket))
			lastHash := bucket.Get([]byte("l"))

			if lastHash == nil {
				fmt.Println("No existing blockchain found. Mining Genesis...")
				genesisTx := NewCoinbaseTX(address, "Genesis Block")
				genesisBlock := &Block{[]*Transaction{genesisTx}, "", "", 0}
				genesisBlock.Mine()
				bucket.Put([]byte(genesisBlock.Hash), genesisBlock.Serialize())
				bucket.Put([]byte("l"), []byte(genesisBlock.Hash))
				newestHash = genesisBlock.Hash
			} else {
				newestHash = string(lastHash)
			}
			return nil
		})
		bInstance = &BlockChain{newestHash, db}
	})
	return bInstance
}

// FindUnspentTransactions: 사용되지 않은 트랜잭션 목록 반환
func (bc *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTXs []Transaction
	spentTXs := make(map[string][]int) // TxID -> []OutIndex (사용된 아웃풋 기록)

	// 주소 문자열을 해시로 변환 (비교용)
	targetPubKeyHash := wallet.Base58ToPubKeyHash(address)

	currHash := bc.NewestHash
	bc.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		for {
			blockBytes := bucket.Get([]byte(currHash))
			if blockBytes == nil {
				break
			}
			block := Deserialize(blockBytes)

			for _, t := range block.Transactions {
				txID := hex.EncodeToString(t.ID)

			Outputs:
				for outIdx, out := range t.Outs {
					// 1. 이미 사용된 기록이 있는지 확인
					if spentTXs[txID] != nil {
						for _, spentOut := range spentTXs[txID] {
							if spentOut == outIdx {
								continue Outputs
							}
						}
					}
					// 2. ⭐️ 수정: 주소 문자열 비교가 아닌 PubKeyHash 비교
					if bytes.Equal(out.PubKeyHash, targetPubKeyHash) {
						unspentTXs = append(unspentTXs, *t)
					}
				}

				// 3. ⭐️ 수정: 누가 썼는지 따지지 않고, 인풋이 가리키는 건 무조건 Spent 처리
				if !t.IsCoinbase() {
					for _, in := range t.Ins {
						inTxID := hex.EncodeToString(in.TxID)
						spentTXs[inTxID] = append(spentTXs[inTxID], in.OutIndex)
					}
				}
			}
			if len(block.PrevHash) == 0 {
				break
			}
			currHash = block.PrevHash
		}
		return nil
	})
	return unspentTXs
}

func (bc *BlockChain) GetBalance(address string) int {
	balance := 0
	pubKeyHash := wallet.Base58ToPubKeyHash(address)
	unspentTXs := bc.FindUnspentTransactions(address)

	for _, tx := range unspentTXs {
		for _, out := range tx.Outs {
			// 내 자물쇠(PubKeyHash)로 잠긴 금액만 합산
			if bytes.Equal(out.PubKeyHash, pubKeyHash) {
				balance += out.Value
			}
		}
	}
	return balance
}

func (bc *BlockChain) AddBlock(txs []*Transaction, miner string) {
	for _, tx := range txs {
		if !tx.IsCoinbase() {
			prevTXs := make(map[string]Transaction)
			for _, vin := range tx.Ins {
				prevTX, err := bc.FindTransaction(vin.TxID)
				if err != nil {
					log.Panic(err)
				}
				prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
			}
			// ⭐️ 여기서 Verify가 실행되며 "주인 확인"과 "서명 확인"을 동시에 함
			if !tx.Verify(prevTXs) {
				log.Panic("Verify Failed: Invalid transaction owner or signature")
			}
		}
	}
	cbTx := NewCoinbaseTX(miner, fmt.Sprintf("Reward for Block: %x", bc.NewestHash))
	blockTxs := append([]*Transaction{cbTx}, txs...)
	newBlock := &Block{blockTxs, "", bc.NewestHash, 0}
	newBlock.Mine()

	bc.DB.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		bucket.Put([]byte(newBlock.Hash), newBlock.Serialize())
		bucket.Put([]byte("l"), []byte(newBlock.Hash))
		bc.NewestHash = newBlock.Hash
		return nil
	})
}

func (bc *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	pubKeyHash := wallet.Base58ToPubKeyHash(address)

Work:
	for _, tx := range bc.FindUnspentTransactions(address) {
		txID := hex.EncodeToString(tx.ID)
		for outIdx, out := range tx.Outs {
			if bytes.Equal(out.PubKeyHash, pubKeyHash) && accumulated < amount {
				accumulated += out.Value
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOutputs
}

func (bc *BlockChain) NewTransaction(w *wallet.Wallet, to string, amount int) *Transaction {
	acc, validOutputs := bc.FindSpendableOutputs(w.GetAddress(), amount)
	if acc < amount {
		log.Panic("Not enough funds")
	}

	var inputs []TxInput
	for txid, outs := range validOutputs {
		txID, _ := hex.DecodeString(txid)
		for _, out := range outs {
			inputs = append(inputs, TxInput{txID, out, nil, w.PublicKey})
		}
	}

	// ⭐️ 수정: 아웃풋 생성 시 PubKeyHash로 잠금
	outputs := []TxOutput{
		{Value: amount, PubKeyHash: wallet.Base58ToPubKeyHash(to)},
	}
	if acc > amount {
		outputs = append(outputs, TxOutput{
			Value:      acc - amount,
			PubKeyHash: wallet.Base58ToPubKeyHash(w.GetAddress()),
		})
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	prevTXs := make(map[string]Transaction)
	for _, vin := range tx.Ins {
		prevTX, _ := bc.FindTransaction(vin.TxID)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	tx.Sign(w.GetPrivateKey(), prevTXs)
	return &tx
}

// 💀 공격용 트랜잭션 함수
func (bc *BlockChain) NewAttackTransaction(hackerWallet *wallet.Wallet, to string, amount int, victimTxIDStr string, victimIdx int) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	victimTxID, _ := hex.DecodeString(victimTxIDStr)

	// [공격] 피해자의 TxID를 쓰지만, PubKey는 해커의 것을 제출함
	input := TxInput{victimTxID, victimIdx, nil, hackerWallet.PublicKey}
	inputs = append(inputs, input)

	// 아웃풋은 해커 주소로 설정
	outputs = append(outputs, TxOutput{amount, wallet.Base58ToPubKeyHash(to)})

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	prevTXs := make(map[string]Transaction)
	prevTX, err := bc.FindTransaction(victimTxID)
	if err != nil {
		log.Panic("Victim TX not found")
	}
	prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX

	// 해커의 비밀키로 서명
	tx.Sign(hackerWallet.GetPrivateKey(), prevTXs)

	return &tx
}

func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
	currHash := bc.NewestHash
	var targetTx Transaction
	err := bc.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		for {
			blockBytes := bucket.Get([]byte(currHash))
			if blockBytes == nil {
				break
			}
			block := Deserialize(blockBytes)
			for _, t := range block.Transactions {
				if bytes.Equal(t.ID, ID) {
					targetTx = *t
					return nil
				}
			}
			if len(block.PrevHash) == 0 {
				break
			}
			currHash = block.PrevHash
		}
		return fmt.Errorf("TX not found")
	})
	return targetTx, err
}

func (bc *BlockChain) AllBlocks() []*Block {
	var blocks []*Block
	currHash := bc.NewestHash
	bc.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		for {
			blockBytes := bucket.Get([]byte(currHash))
			if blockBytes == nil {
				break
			}
			block := Deserialize(blockBytes)
			blocks = append(blocks, block)
			if len(block.PrevHash) == 0 {
				break
			}
			currHash = block.PrevHash
		}
		return nil
	})
	return blocks
}
