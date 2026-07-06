package protocol

import (
	"fmt"
	"io"
	"time"
)

type SecureConn struct {
	r       io.Reader
	w       io.Writer
	sendKey []byte
	recvKey []byte
	sendSeq uint64
	recvSeq uint64
}

func NewSecureConn(rw io.ReadWriter, sendKey []byte, recvKey []byte) *SecureConn {
	return &SecureConn{
		r:       rw,
		w:       rw,
		sendKey: sendKey,
		recvKey: recvKey,
	}
}

func (c *SecureConn) ReadMessage() (Message, error) {
	outer, err := ReadMessage(c.r)
	if err != nil {
		return Message{}, err
	}
	if outer.Type != MessageTypeEncrypted {
		return Message{}, fmt.Errorf("expected encrypted message, got %q", outer.Type)
	}

	encrypted, err := PayloadTo[EncryptedMessage](outer.Payload)
	if err != nil {
		return Message{}, err
	}
	if outer.Seq != 0 && outer.Seq != encrypted.Seq {
		return Message{}, fmt.Errorf("encrypted sequence mismatch: outer=%d payload=%d", outer.Seq, encrypted.Seq)
	}
	if encrypted.Seq <= c.recvSeq {
		return Message{}, fmt.Errorf("encrypted sequence replay: seq=%d last=%d", encrypted.Seq, c.recvSeq)
	}

	plaintext, err := Open(c.recvKey, encrypted)
	if err != nil {
		return Message{}, err
	}
	message, err := DecodeMessage(plaintext)
	if err != nil {
		return Message{}, err
	}
	c.recvSeq = encrypted.Seq
	return message, nil
}

func (c *SecureConn) WriteMessage(message Message) error {
	c.sendSeq++
	raw, err := EncodeMessage(message)
	if err != nil {
		return err
	}
	encrypted, err := Seal(c.sendKey, c.sendSeq, raw)
	if err != nil {
		return err
	}
	payload, err := PayloadFrom(encrypted)
	if err != nil {
		return err
	}
	return WriteMessage(c.w, Message{
		Type:      MessageTypeEncrypted,
		Seq:       c.sendSeq,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
}
