package tokencount_test

import (
	"testing"

	"github.com/cloud-byte-consulting/teo/internal/tokencount"
)

func TestCount(t *testing.T) {
	n, err := tokencount.Count("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Fatalf("token count = %d, want > 0", n)
	}
}
