package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const MaxFrameSize = 4 * 1024 * 1024

func ReadFrame(r io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}

	n := binary.BigEndian.Uint32(lenBuf[:])
	if n == 0 || n > MaxFrameSize {
		return nil, fmt.Errorf("invalid frame length: %d", n)
	}

	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	return payload, nil
}
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) == 0 || len(payload) > MaxFrameSize {
		return fmt.Errorf("invalid frame length: %d", len(payload))
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}

	_, err := w.Write(payload)
	return err
}
