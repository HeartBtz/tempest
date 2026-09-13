package bencode

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeDepthLimit(t *testing.T) {
	withinLimit := strings.Repeat("l", maxDecodeDepth-1) + "0:" + strings.Repeat("e", maxDecodeDepth-1)
	if _, err := Decode(strings.NewReader(withinLimit)); err != nil {
		t.Fatalf("decode at depth limit: %v", err)
	}

	overLimit := "l" + withinLimit + "e"
	if _, err := Decode(strings.NewReader(overLimit)); !errors.Is(err, ErrDepthLimit) {
		t.Fatalf("decode over depth limit error = %v, want ErrDepthLimit", err)
	}
}

func TestDecodeNodeLimit(t *testing.T) {
	var input strings.Builder
	input.Grow(maxDecodeNodes*2 + 2)
	input.WriteByte('l')
	for range maxDecodeNodes {
		input.WriteString("0:")
	}
	input.WriteByte('e')

	if _, err := Decode(strings.NewReader(input.String())); !errors.Is(err, ErrNodeLimit) {
		t.Fatalf("decode over node limit error = %v, want ErrNodeLimit", err)
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	if _, err := Decode(strings.NewReader("0:0:")); !errors.Is(err, ErrInvalidBencode) {
		t.Fatalf("decode trailing data error = %v, want ErrInvalidBencode", err)
	}
}
