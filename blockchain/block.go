package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
)

const difficulty = 2

type Block struct {
	Transactions []*Transaction
	Hash         string
	PrevHash     string
	Nonce        int
}

func (b *Block) Mine() {
	target := ""
	for i := 0; i < difficulty; i++ {
		target += "0"
	}

	for {
		var txHashes [][]byte
		for _, tx := range b.Transactions {
			txHashes = append(txHashes, tx.ID)
		}
		data := fmt.Sprintf("%x%s%d", txHashes, b.PrevHash, b.Nonce)
		hash := sha256.Sum256([]byte(data))
		hashStr := fmt.Sprintf("%x", hash)

		if hashStr[:difficulty] == target {
			b.Hash = hashStr
			break
		}
		b.Nonce++
	}
}

func (b *Block) Serialize() []byte {
	var res bytes.Buffer
	gob.NewEncoder(&res).Encode(b)
	return res.Bytes()
}

func Deserialize(data []byte) *Block {
	var block Block
	gob.NewDecoder(bytes.NewReader(data)).Decode(&block)
	return &block
}
