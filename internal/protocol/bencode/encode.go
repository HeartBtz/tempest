package bencode

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
)

func Encode(w io.Writer, val interface{}) error {
	switch v := val.(type) {
	case int:
		return encodeInt(w, int64(v))
	case int64:
		return encodeInt(w, v)
	case string:
		return encodeString(w, []byte(v))
	case []byte:
		return encodeString(w, v)
	case []interface{}:
		return encodeList(w, v)
	case map[string]interface{}:
		return encodeDict(w, v)
	default:
		return fmt.Errorf("unsupported type for bencode: %T", val)
	}
}

func EncodeToBytes(val interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := Encode(&buf, val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeInt(w io.Writer, val int64) error {
	_, err := fmt.Fprintf(w, "i%de", val)
	return err
}

func encodeString(w io.Writer, val []byte) error {
	_, err := fmt.Fprintf(w, "%d:", len(val))
	if err != nil {
		return err
	}
	_, err = w.Write(val)
	return err
}

func encodeList(w io.Writer, val []interface{}) error {
	if _, err := w.Write([]byte("l")); err != nil {
		return err
	}
	for _, item := range val {
		if err := Encode(w, item); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("e"))
	return err
}

func encodeDict(w io.Writer, val map[string]interface{}) error {
	if _, err := w.Write([]byte("d")); err != nil {
		return err
	}

	// Keys must be sorted
	keys := make([]string, 0, len(val))
	for k := range val {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if err := encodeString(w, []byte(k)); err != nil {
			return err
		}
		if err := Encode(w, val[k]); err != nil {
			return err
		}
	}

	_, err := w.Write([]byte("e"))
	return err
}

func EncodeDictRaw(w io.Writer, data []byte, key string) ([]byte, error) {
	_ = strconv.Itoa(len(key))
	return data, nil
}
