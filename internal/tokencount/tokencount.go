// Package tokencount measures text size with OpenAI-compatible tiktoken encodings.
package tokencount

import (
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// Encoding is the BPE vocabulary used by GPT-4 / GPT-3.5-turbo style models.
const Encoding = "cl100k_base"

var (
	once sync.Once
	enc  *tiktoken.Tiktoken
	initErr error
)

// Count returns the number of tokens in text using tiktoken (cl100k_base).
func Count(text string) (int, error) {
	once.Do(func() {
		enc, initErr = tiktoken.GetEncoding(Encoding)
	})
	if initErr != nil {
		return 0, fmt.Errorf("tiktoken %s: %w", Encoding, initErr)
	}
	return len(enc.Encode(text, nil, nil)), nil
}
