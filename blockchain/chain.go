package blockchain

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"github.com/boltdb/bolt"
)

const (
	blocksBucket = "blocks"
)

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
			bucket, _ := tx.CreateBucketIfNotExists([]byte("blocks"))
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

// UTXO 검색 (내 주소로 된 안 쓴 돈 찾기)
func (bc *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTXs []Transaction
	spentTXs := make(map[string][]int)

	currHash := bc.NewestHash
	bc.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("blocks"))
		for {
			block := Deserialize(bucket.Get([]byte(currHash)))
			for _, t := range block.Transactions {
				txID := hex.EncodeToString(t.ID)
			Outputs:
				for outIdx, out := range t.Outs {
					if spentTXs[txID] != nil {
						for _, spentOut := range spentTXs[txID] {
							if spentOut == outIdx {
								continue Outputs
							}
						}
					}
					if out.PubKey == address {
						unspentTXs = append(unspentTXs, *t)
					}
				}
				if !t.IsCoinbase() {
					for _, in := range t.Ins {
						if in.FromAddr == address {
							inTxID := hex.EncodeToString(in.TxID)
							spentTXs[inTxID] = append(spentTXs[inTxID], in.OutIndex)
						}
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
	utxos := bc.FindUnspentTransactions(address)
	for _, tx := range utxos {
		for _, out := range tx.Outs {
			if out.PubKey == address {
				balance += out.Value
			}
		}
	}
	return balance
}

func (bc *BlockChain) AddBlock(txs []*Transaction, miner string) {
	// 1. 이 블록을 위한 코인베이스 트랜잭션 생성 (채굴 보상 50)
	cbTx := NewCoinbaseTX(miner, fmt.Sprintf("Reward for Block: %s", bc.NewestHash))

	// 2. 코인베이스를 맨 앞에 두고, 일반 거래들을 뒤에 붙임
	var blockTxs []*Transaction
	blockTxs = append(blockTxs, cbTx)
	blockTxs = append(blockTxs, txs...)

	// 3. 이 트랜잭션 뭉치를 가지고 새로운 블록 객체 생성
	newBlock := &Block{
		Transactions: blockTxs,
		PrevHash:     bc.NewestHash, // 이전 블록과 연결
		Nonce:        0,
	}

	newBlock.Mine()

	bc.DB.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("blocks"))
		bucket.Put([]byte(newBlock.Hash), newBlock.Serialize())
		bucket.Put([]byte("l"), []byte(newBlock.Hash))
		bc.NewestHash = newBlock.Hash
		return nil
	})
}

// FindSpendableOutputs: 송금에 필요한 충분한 액수의 UTXO들을 찾아 반환
func (bc *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	unspentTXs := bc.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID)
		for outIdx, out := range tx.Outs {
			if out.PubKey == address && accumulated < amount {
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

// NewTransaction: 일반적인 송금 트랜잭션 생성
func (bc *BlockChain) NewTransaction(from, to string, amount int) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	acc, validOutputs := bc.FindSpendableOutputs(from, amount)

	if acc < amount {
		log.Panic("에러: 잔액이 부족합니다")
	}

	// Input 생성 (찾은 UTXO들을 소모)
	for txid, outs := range validOutputs {
		txID, _ := hex.DecodeString(txid)
		for _, out := range outs {
			input := TxInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}

	// Output 생성 (상대방에게 전송)
	outputs = append(outputs, TxOutput{amount, to})

	// 거스름돈 처리
	if acc > amount {
		outputs = append(outputs, TxOutput{acc - amount, from})
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()
	return &tx
}

func (bc *BlockChain) AllBlocks() []*Block {
	var blocks []*Block
	currHash := bc.NewestHash

	bc.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))

		// 1. 최신 해시부터 시작해서 PrevHash가 없을 때까지 루프
		for {
			blockBytes := bucket.Get([]byte(currHash))
			block := Deserialize(blockBytes)
			blocks = append(blocks, block)

			// 2. 제네시스 블록(이전 해시가 없음)에 도달하면 종료
			if len(block.PrevHash) == 0 {
				break
			}
			currHash = block.PrevHash
		}
		return nil
	})
	return blocks
}
