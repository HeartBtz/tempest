package bencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

var (
	ErrInvalidBencode = errors.New("invalid bencode data")
	ErrUnexpectedEOF  = errors.New("unexpected end of bencode data")
)

func Decode(r io.Reader) (interface{}, error) {
	br := bufio.NewReader(r)
	return decodeValue(br)
}

func decodeValue(r *bufio.Reader) (interface{}, error) {
	b, err := r.Peek(1)
	if err != nil {
		return nil, ErrUnexpectedEOF
	}

	switch {
	case b[0] == 'i':
		return decodeInt(r)
	case b[0] == 'l':
		return decodeList(r)
	case b[0] == 'd':
		return decodeDict(r)
	case b[0] >= '0' && b[0] <= '9':
		return decodeString(r)
	default:
		return nil, ErrInvalidBencode
	}
}

const maxIntLength = 32   // max digits for bencode integer
const maxStringLen = 64 * 1024 * 1024 // 64MB max string length

func decodeInt(r *bufio.Reader) (int64, error) {
	// consume 'i'
	if _, err := r.ReadByte(); err != nil {
		return 0, err
	}

	numStr := make([]byte, 0, 20)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, ErrUnexpectedEOF
		}
		if b == 'e' {
			break
		}
		if len(numStr) >= maxIntLength {
			return 0, ErrInvalidBencode
		}
		numStr = append(numStr, b)
	}

	return strconv.ParseInt(string(numStr), 10, 64)
}

func decodeString(r *bufio.Reader) ([]byte, error) {
	lenStr := make([]byte, 0, 10)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		if b == ':' {
			break
		}
		if len(lenStr) >= 10 {
			return nil, ErrInvalidBencode
		}
		lenStr = append(lenStr, b)
	}

	length, err := strconv.Atoi(string(lenStr))
	if err != nil {
		return nil, ErrInvalidBencode
	}

	if length < 0 || length > maxStringLen {
		return nil, fmt.Errorf("bencode string length %d exceeds maximum %d", length, maxStringLen)
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, ErrUnexpectedEOF
	}

	return buf, nil
}

func decodeList(r *bufio.Reader) ([]interface{}, error) {
	// consume 'l'
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}

	var list []interface{}
	for {
		b, err := r.Peek(1)
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		if b[0] == 'e' {
			r.ReadByte()
			break
		}

		val, err := decodeValue(r)
		if err != nil {
			return nil, err
		}
		list = append(list, val)
	}

	return list, nil
}

func decodeDict(r *bufio.Reader) (map[string]interface{}, error) {
	// consume 'd'
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}

	dict := make(map[string]interface{})
	for {
		b, err := r.Peek(1)
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		if b[0] == 'e' {
			r.ReadByte()
			break
		}

		keyBytes, err := decodeString(r)
		if err != nil {
			return nil, err
		}

		val, err := decodeValue(r)
		if err != nil {
			return nil, err
		}

		dict[string(keyBytes)] = val
	}

	return dict, nil
}
