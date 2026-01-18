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

//// 3. 서명 기반 송금 API: /send?from=ADDR&to=ADDR&amount=10
//func sendHandler(w http.ResponseWriter, r *http.Request) {
//	from := r.URL.Query().Get("from")
//	to := r.URL.Query().Get("to")
//	amount, _ := strconv.Atoi(r.URL.Query().Get("amount"))
//
//	// 1. 지갑 저장소에서 비공개키가 포함된 지갑 객체 찾기
//	fromWallet, ok := wallets.Wallets[from]
//	if !ok {
//		http.Error(w, "Sender wallet not found in server memory", 400)
//		return
//	}
//
//	// 2. 트랜잭션 생성 및 내부 서명(Sign) 실행
//	tx := bc.NewTransaction(fromWallet, to, amount)
//
//	// 3. 블록 추가 및 내부 검증(Verify) 실행
//	bc.AddBlock([]*blockchain.Transaction{tx}, "gopher")
//
//	fmt.Fprintf(w, "Success! %d coins sent from %s to %s\n", amount, from, to)
//}

// 4. 전체 블록 조회 API: /explorer
func explorerHandler(w http.ResponseWriter, r *http.Request) {
	blocks := bc.AllBlocks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blocks)
}

type Server struct{}

func sendHandler(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	amountStr := r.URL.Query().Get("amount")
	amount, _ := strconv.Atoi(amountStr)

	// ✅ 수정 포인트: 패키지 함수를 호출하지 않고,
	// 위에서 이미 선언한 전역 변수 'wallets'에서 직접 찾습니다.
	if wallets == nil || wallets.Wallets == nil {
		http.Error(w, "Wallets not initialized", http.StatusInternalServerError)
		return
	}

	hackerWallet := wallets.Wallets[from] // 전역 변수 wallets 안의 맵(Wallets) 참조
	if hackerWallet == nil {
		http.Error(w, "Hacker wallet not found in server memory", http.StatusBadRequest)
		return
	}

	// 피해자 데이터 (제네시스 블록의 50코인 타겟)
	victimTxID := "25297a4164a109212c93ad14834f0d6ec62eaf9d4bdcdd8ed941900e5c73c2b9"
	victimIdx := 0

	// 현재 실행 중인 블록체인 인스턴스 사용
	// (이미 전역 변수 bc가 있으니 그걸 써도 됩니다)
	targetBc := blockchain.GetBlockchain("")

	// 공격용 트랜잭션 생성 (해커의 지갑으로 서명하지만 인풋은 피해자의 것)
	tx := targetBc.NewAttackTransaction(hackerWallet, to, amount, victimTxID, victimIdx)

	// 블록체인에 추가 (여기서 Verify가 실행됨)
	targetBc.AddBlock([]*blockchain.Transaction{tx}, "miner")

	fmt.Fprintf(w, "💀 Attack Attempted! Check if %s's balance increased", to)
}
