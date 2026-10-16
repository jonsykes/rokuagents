package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.jsonl")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		size  int64
		want  string
		whole bool
	}{
		{4, "6789", false},
		{10, "0123456789", true},
		{64, "0123456789", true},
	}
	for _, tt := range tests {
		got, whole, err := ReadTail(path, tt.size)
		if err != nil || string(got) != tt.want || whole != tt.whole {
			t.Errorf("ReadTail(%d) = %q, %v, %v; want %q, %v", tt.size, got, whole, err, tt.want, tt.whole)
		}
	}
}

func TestReadTailOfAMissingFile(t *testing.T) {
	if _, _, err := ReadTail(filepath.Join(t.TempDir(), "missing"), 4); !os.IsNotExist(err) {
		t.Errorf("err = %v, want not exist", err)
	}
}
