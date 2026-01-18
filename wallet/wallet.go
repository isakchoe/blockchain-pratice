package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"
	"math/big"

	"github.com/mr-tron/base58"
)

const version = byte(0x00)

type Wallet struct {
	RawPrivateKey []byte
	PublicKey     []byte
}

func NewWallet() *Wallet {
	private, public := newPair()
	return &Wallet{private, public}
}

func newPair() ([]byte, []byte) {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	// 1. PrivateKey의 D값 추출
	privBytes := private.D.Bytes()

	// 2. PublicKey의 X, Y 좌표를 각각 32바이트로 추출하여 합침
	// 단순히 Bytes()만 쓰면 앞자리가 0일 때 길이가 짧아질 수 있으므로 고정 길이로 만듭니다.
	pubBytes := append(padBytes(private.PublicKey.X.Bytes(), 32), padBytes(private.PublicKey.Y.Bytes(), 32)...)

	return privBytes, pubBytes
}

// GetPrivateKey: 바이트로부터 ecdsa 객체를 조립
func (w Wallet) GetPrivateKey() ecdsa.PrivateKey {
	curve := elliptic.P256()

	d := new(big.Int).SetBytes(w.RawPrivateKey)

	// X와 Y를 정확히 반(32바이트)씩 나눠서 복원
	x := new(big.Int).SetBytes(w.PublicKey[:32])
	y := new(big.Int).SetBytes(w.PublicKey[32:])

	return ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}
}

// ⭐️ 데이터 길이가 짧을 경우 앞에 0을 채워주는 헬퍼 함수
func padBytes(b []byte, length int) []byte {
	padded := make([]byte, length)
	copy(padded[length-len(b):], b)
	return padded
}

func (w Wallet) GetAddress() string {
	pubKeyHash := HashPubKey(w.PublicKey)
	versionedPayload := append([]byte{version}, pubKeyHash...)
	checksum := Checksum(versionedPayload)
	fullPayload := append(versionedPayload, checksum...)
	address := base58.Encode(fullPayload)
	return address
}
