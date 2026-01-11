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

func (bc *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTXs []Transaction
	spentTXs := make(map[string][]int)

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
					if spentTXs[txID] != nil {
						for _, spentOut := range spentTXs[txID] {
							if spentOut == outIdx {
								continue Outputs
							}
						}
					}
					// TxOutput의 PubKey와 현재 조회 주소 비교
					if string(out.PubKey) == address {
						unspentTXs = append(unspentTXs, *t)
					}
				}
				if !t.IsCoinbase() {
					for _, in := range t.Ins {
						// TxInput의 PubKey를 주소로 간주하여 비교 (단순화 버전)
						if string(in.PubKey) == address {
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
	for _, tx := range bc.FindUnspentTransactions(address) {
		for _, out := range tx.Outs {
			if string(out.PubKey) == address {
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
			if !tx.Verify(prevTXs) {
				log.Panic("Verify Failed")
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
Work:
	for _, tx := range bc.FindUnspentTransactions(address) {
		txID := hex.EncodeToString(tx.ID)
		for outIdx, out := range tx.Outs {
			if string(out.PubKey) == address && accumulated < amount {
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

	outputs := []TxOutput{{amount, []byte(to)}}
	if acc > amount {
		outputs = append(outputs, TxOutput{acc - amount, []byte(w.GetAddress())})
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	prevTXs := make(map[string]Transaction)
	for _, vin := range tx.Ins {
		prevTX, _ := bc.FindTransaction(vin.TxID)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	tx.Sign(w.PrivateKey, prevTXs)
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
