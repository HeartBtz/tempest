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
	ErrDepthLimit     = errors.New("bencode nesting depth exceeds limit")
	ErrNodeLimit      = errors.New("bencode decoded node count exceeds limit")
)

const (
	maxDecodeDepth = 100
	maxDecodeNodes = 100_000
	maxIntLength   = 32               // max digits for bencode integer
	maxStringLen   = 64 * 1024 * 1024 // 64MB max string length
)

func Decode(r io.Reader) (interface{}, error) {
	d := decoder{r: bufio.NewReader(r)}
	value, err := d.decodeValue(1)
	if err != nil {
		return nil, err
	}
	if _, err := d.r.Peek(1); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: trailing data", ErrInvalidBencode)
	}
	return value, nil
}

type decoder struct {
	r     *bufio.Reader
	nodes int
}

func (d *decoder) decodeValue(depth int) (interface{}, error) {
	if depth > maxDecodeDepth {
		return nil, fmt.Errorf("%w (maximum %d)", ErrDepthLimit, maxDecodeDepth)
	}
	if err := d.consumeNode(); err != nil {
		return nil, err
	}

	b, err := d.r.Peek(1)
	if err != nil {
		return nil, ErrUnexpectedEOF
	}

	switch {
	case b[0] == 'i':
		return decodeInt(d.r)
	case b[0] == 'l':
		return d.decodeList(depth)
	case b[0] == 'd':
		return d.decodeDict(depth)
	case b[0] >= '0' && b[0] <= '9':
		return decodeString(d.r)
	default:
		return nil, ErrInvalidBencode
	}
}

func (d *decoder) consumeNode() error {
	d.nodes++
	if d.nodes > maxDecodeNodes {
		return fmt.Errorf("%w (maximum %d)", ErrNodeLimit, maxDecodeNodes)
	}
	return nil
}

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

func (d *decoder) decodeList(depth int) ([]interface{}, error) {
	// consume 'l'
	if _, err := d.r.ReadByte(); err != nil {
		return nil, err
	}

	var list []interface{}
	for {
		b, err := d.r.Peek(1)
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		if b[0] == 'e' {
			_, _ = d.r.ReadByte()
			break
		}

		val, err := d.decodeValue(depth + 1)
		if err != nil {
			return nil, err
		}
		list = append(list, val)
	}

	return list, nil
}

func (d *decoder) decodeDict(depth int) (map[string]interface{}, error) {
	// consume 'd'
	if _, err := d.r.ReadByte(); err != nil {
		return nil, err
	}

	dict := make(map[string]interface{})
	for {
		b, err := d.r.Peek(1)
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		if b[0] == 'e' {
			_, _ = d.r.ReadByte()
			break
		}

		if err := d.consumeNode(); err != nil {
			return nil, err
		}
		keyBytes, err := decodeString(d.r)
		if err != nil {
			return nil, err
		}

		val, err := d.decodeValue(depth + 1)
		if err != nil {
			return nil, err
		}

		dict[string(keyBytes)] = val
	}

	return dict, nil
}
