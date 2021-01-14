package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/rand"
	"io"
)

func createHash(key string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hasher.Sum(nil)
}

func Encrypt(data []byte, passphrase string) (b []byte, err error) {
	var block cipher.Block
	if block, err = aes.NewCipher(createHash(passphrase)); err != nil {
		return
	}
	var gcm cipher.AEAD
	if gcm, err = cipher.NewGCM(block); err != nil {
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}
	b = gcm.Seal(nonce, nonce, data, nil)
	return
}

func Decrypt(data []byte, passphrase string) (b []byte, err error) {
	key := createHash(passphrase)
	var block cipher.Block
	if block, err = aes.NewCipher(key); err != nil {
		return
	}
	var gcm cipher.AEAD
	if gcm, err = cipher.NewGCM(block); err != nil {
		return
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	if b, err = gcm.Open(nil, nonce, ciphertext, nil); err != nil {
		return
	}
	return
}
