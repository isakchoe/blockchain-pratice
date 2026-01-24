package wallet

import (
	"crypto/sha256"
	"log"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

// HashPubKey: 공개키를 SHA256 + RIPEMD160으로 해싱
func HashPubKey(pubKey []byte) []byte {
	publicSHA256 := sha256.Sum256(pubKey)

	RIPEMD160Hasher := ripemd160.New()
	_, err := RIPEMD160Hasher.Write(publicSHA256[:])
	if err != nil {
		log.Panic(err)
	}
	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)

	return publicRIPEMD160
}

// Checksum: 버전+해시 데이터에 대해 이중 SHA256을 수행하고 앞 4바이트 추출
func Checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])

	return secondSHA[:4]
}

// Base58ToPubKeyHash: 주소(string)를 넣으면 내부의 PubKeyHash([]byte)를 추출하는 함수
func Base58ToPubKeyHash(address string) []byte {
	// 1. Base58로 인코딩된 주소를 디코딩합니다.
	pubKeyHash := base58.Decode(address)

	// 2. 비트코인 주소 형식상 앞의 1바이트(버전)와 뒤의 4바이트(체크섬)를 떼어냅니다.
	// 보통 [Version(1) + PubKeyHash(20) + Checksum(4)] 구조입니다.
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

	return pubKeyHash
}
