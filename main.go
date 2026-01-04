package main

import (
	"blockchain/blockchain"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

var bc *blockchain.BlockChain

// 잔액 조회 핸들러: /balance?address=Gopher
func balanceHandler(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	balance := bc.GetBalance(address)
	fmt.Fprintf(w, "Address: %s, Balance: %d\n", address, balance)
}

// 송금 핸들러: /send?from=Gopher&to=busak&amount=10
func sendHandler(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	amountStr := r.URL.Query().Get("amount")
	amount, _ := strconv.Atoi(amountStr)

	// 1. 트랜잭션 생성
	tx := bc.NewTransaction(from, to, amount)
	// 2. 블록에 담아 채굴 (현 단계에선 전송 시 바로 채굴됨)
	bc.AddBlock([]*blockchain.Transaction{tx}, "Gopher")

	fmt.Fprintf(w, "Success: %d coins sent from %s to %s\n", amount, from, to)
}

func explorerHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 모든 블록 가져오기
	blocks := bc.AllBlocks()

	// 2. 응답 헤더를 JSON으로 설정
	w.Header().Set("Content-Type", "application/json")

	// 3. JSON 데이터를 보기 좋게 인덴트(들여쓰기) 처리하여 전송
	json.NewEncoder(w).Encode(blocks)
}

func main() {
	// Gopher에게 제네시스 보상을 주며 시작
	bc = blockchain.GetBlockchain("Gopher")
	defer bc.DB.Close()

	http.HandleFunc("/balance", balanceHandler)
	http.HandleFunc("/send", sendHandler)
	http.HandleFunc("/explorer", explorerHandler)

	fmt.Println("Blockchain Server started on :4000")
	http.ListenAndServe(":4000", nil)
}
