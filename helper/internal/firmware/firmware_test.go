package firmware

import (
	"testing"
	"testing/fstest"
)

func TestEmbedded(t *testing.T) {
	image := []byte{0xe9, 0x03, 0x02, 0x2f}
	tests := []struct {
		name        string
		files       fstest.MapFS
		board       string
		wantOK      bool
		wantVersion string
	}{
		{"image and version", fstest.MapFS{
			"images/managents-firmware-e32r40t.bin":     {Data: image},
			"images/managents-firmware-e32r40t.version": {Data: []byte("0.2.0\n")},
		}, "e32r40t", true, "0.2.0"},
		{"git describe version", fstest.MapFS{
			"images/managents-firmware-e32r40t.bin":     {Data: image},
			"images/managents-firmware-e32r40t.version": {Data: []byte("f91cdc5-dirty\n")},
		}, "e32r40t", true, "f91cdc5-dirty"},
		{"image without version", fstest.MapFS{
			"images/managents-firmware-e32r40t.bin": {Data: image},
		}, "e32r40t", true, ""},
		{"other board", fstest.MapFS{
			"images/managents-firmware-e32r40t.bin": {Data: image},
		}, "e32n40t", false, ""},
		{"version without image", fstest.MapFS{
			"images/managents-firmware-e32r40t.version": {Data: []byte("0.2.0\n")},
		}, "e32r40t", false, ""},
		{"empty image", fstest.MapFS{
			"images/managents-firmware-e32r40t.bin": {},
		}, "e32r40t", false, ""},
		{"no images", fstest.MapFS{"images/README.md": {Data: []byte("x")}}, "e32r40t", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, version, ok := embedded(tt.files, tt.board)
			if ok != tt.wantOK || version != tt.wantVersion {
				t.Fatalf("embedded() = ok %v, version %q; want %v, %q", ok, version, tt.wantOK, tt.wantVersion)
			}
			if ok && string(got) != string(image) || !ok && got != nil {
				t.Errorf("image = %x", got)
			}
		})
	}
}

func TestEmbeddedKnowsOnlyItsBoards(t *testing.T) {
	// Whatever this build embeds, a board nobody built firmware for has none.
	if _, _, ok := Embedded("no-such-board"); ok {
		t.Error("Embedded reports an image for an unknown board")
	}
}
