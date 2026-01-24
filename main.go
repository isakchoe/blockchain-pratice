package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"blockchain/blockchain"
	"blockchain/wallet"
)

var bc *blockchain.BlockChain
var wallets *wallet.Wallets // 테스트용 지갑 저장소

func main() {

	// 1 저장된 지갑들 불러오기
	var err error
	wallets, err = wallet.NewWallets()
	if err != nil {
		fmt.Println("No wallets found, creating miner wallet")
		wallets.CreateWallet()
		wallets.SaveToFile()
	}

	// 2 채굴자 주소 가져오기
	var minerAddr string
	for addr := range wallets.Wallets {
		minerAddr = addr
		break
	}

	fmt.Printf("Miner Address: %s\n", minerAddr)
	bc = blockchain.GetBlockchain(minerAddr)
	defer bc.DB.Close()

	http.HandleFunc("/wallet", createWalletHandler)
	http.HandleFunc("/balance", balanceHandler)
	http.HandleFunc("/send", sendHandler)
	http.HandleFunc("/explorer", explorerHandler)

	fmt.Println("Blockchain Server started on :4000")
	http.ListenAndServe(":4000", nil)
}

// 1. 새로운 지갑 생성 API: /wallet
func createWalletHandler(w http.ResponseWriter, r *http.Request) {
	addr := wallets.CreateWallet()
	wallets.SaveToFile()
	fmt.Fprintf(w, "New Wallet Created!\nAddress: %s\n", addr)
}

// 2. 잔액 조회 API: /balance?address=ADDR
func balanceHandler(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	balance := bc.GetBalance(address)
	fmt.Fprintf(w, "Address: %s\nBalance: %d\n", address, balance)
}

// 3. 서명 기반 송금 API: /send?from=ADDR&to=ADDR&amount=10
func sendHandler(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	amount, _ := strconv.Atoi(r.URL.Query().Get("amount"))

	// 1. 지갑 저장소에서 비공개키가 포함된 지갑 객체 찾기
	fromWallet, ok := wallets.Wallets[from]
	if !ok {
		http.Error(w, "Sender wallet not found in server memory", 400)
		return
	}

	// 2. 트랜잭션 생성 및 내부 서명(Sign) 실행
	tx := bc.NewTransaction(fromWallet, to, amount)

	// 3. 블록 추가 및 내부 검증(Verify) 실행
	bc.AddBlock([]*blockchain.Transaction{tx}, "gopher")

	fmt.Fprintf(w, "Success! %d coins sent from %s to %s\n", amount, from, to)
}

// 4. 전체 블록 조회 API: /explorer
func explorerHandler(w http.ResponseWriter, r *http.Request) {
	blocks := bc.AllBlocks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blocks)
}
