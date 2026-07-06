package protocol

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

const (
	SessionKeySize = chacha20poly1305.KeySize
	HandshakeSize  = 32
)

type SessionKeys struct {
	AgentToMaster []byte
	MasterToAgent []byte
}

type EncryptedMessage struct {
	Seq        uint64 `json:"seq"`
	Ciphertext []byte `json:"ciphertext"`
}

func RandomBytes(size int) ([]byte, error) {
	out := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, out)
	return out, err
}

func DecodeSecretKey(value string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(raw) < SessionKeySize {
		return nil, fmt.Errorf("secret key must be at least %d bytes", SessionKeySize)
	}
	return raw, nil
}

func DeriveSessionKeys(secretKey, agentNonce, masterNonce []byte) (SessionKeys, error) {
	salt := append(append([]byte{}, agentNonce...), masterNonce...)
	reader := hkdf.New(sha256.New, secretKey, salt, []byte("rivo/probe-session/v1"))

	keys := SessionKeys{
		AgentToMaster: make([]byte, SessionKeySize),
		MasterToAgent: make([]byte, SessionKeySize),
	}
	if _, err := io.ReadFull(reader, keys.AgentToMaster); err != nil {
		return SessionKeys{}, err
	}
	if _, err := io.ReadFull(reader, keys.MasterToAgent); err != nil {
		return SessionKeys{}, err
	}
	if hmac.Equal(keys.AgentToMaster, keys.MasterToAgent) {
		return SessionKeys{}, fmt.Errorf("derived duplicate session keys")
	}

	return keys, nil
}

func Seal(key []byte, seq uint64, plaintext []byte) (EncryptedMessage, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return EncryptedMessage{}, err
	}

	nonce := nonceFromSeq(seq)
	ciphertext := aead.Seal(nil, nonce, plaintext, aadFromSeq(seq))

	return EncryptedMessage{Seq: seq, Ciphertext: ciphertext}, nil
}

func Open(key []byte, message EncryptedMessage) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	return aead.Open(nil, nonceFromSeq(message.Seq), message.Ciphertext, aadFromSeq(message.Seq))
}

func nonceFromSeq(seq uint64) []byte {
	nonce := make([]byte, chacha20poly1305.NonceSize)
	binary.BigEndian.PutUint64(nonce[4:], seq)
	return nonce
}

func aadFromSeq(seq uint64) []byte {
	var aad [8]byte
	binary.BigEndian.PutUint64(aad[:], seq)
	return aad[:]
}
