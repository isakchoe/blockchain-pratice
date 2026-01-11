package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
)

type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

// 새로운 지갑 생성
func NewWallet() *Wallet {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		log.Panic(err)
	}
	// 공개키는 X, Y 좌표값을 합친 바이트 배열로 저장
	public := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)

	return &Wallet{*private, public}
}

// GetAddress: 공개키를 해싱하여 우리가 흔히 보는 '주소' 형태를 만듭니다.
// (실제 비트코인은 Base58 체크섬을 쓰지만, 여기선 이해를 위해 간단히 SHA256 해시를 사용합니다)
func (w Wallet) GetAddress() string {
	pubKeyHash := sha256.Sum256(w.PublicKey)
	return fmt.Sprintf("%x", pubKeyHash)
}
