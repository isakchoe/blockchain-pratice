package wallet

import (
	"crypto/sha256"
	"log"

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
