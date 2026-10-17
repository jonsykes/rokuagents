package protocol

import (
	"os"
	"regexp"
	"testing"
)

// The firmware and the helper are released together under one version, so
// the "## [X.Y.Z]" headings of the changelog are the firmware versions that
// exist. The firmware started at firstFirmware, which therefore counts even
// while the changelog lists no release.
const (
	changelog     = "../../../CHANGELOG.md"
	firstFirmware = "0.1.0"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in     string
		want   Release
		wantOK bool
	}{
		{"0.1.0", Release{0, 1, 0}, true},
		{"v0.2.0", Release{0, 2, 0}, true},
		{"10.20.30", Release{10, 20, 30}, true},
		{"0.2.0-3-gabc1234", Release{0, 2, 0}, true}, // git describe after a tag
		{"0.2.0-rc.1", Release{0, 2, 0}, true},
		{"0.2.0+dirty", Release{0, 2, 0}, true},
		{"0.0.0-dev", Release{}, false}, // an untagged build
		{"0.0.0", Release{}, false},
		{"f91cdc5", Release{}, false}, // git describe without a tag
		{"f91cdc5-dirty", Release{}, false},
		{"", Release{}, false},
		{"dev", Release{}, false},
		{"0.1", Release{}, false},
		{"0.1.0.1", Release{}, false},
		{"0.1.x", Release{}, false},
		{"0.-1.0", Release{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseVersion(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestOlderThan(t *testing.T) {
	tests := []struct {
		version, other string
		want           bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.1", "0.2.0", false},
		{"0.9.0", "0.10.0", true}, // numbers, not text
		{"v0.1.0", "0.2.0", true},
		{"0.2.0-3-gabc1234", "0.2.0", false},
		{"0.1.9-3-gabc1234", "0.2.0", true},
		// Versions that are not releases are never older, nor is anything
		// older than them.
		{"f91cdc5", "0.1.0", false},
		{"0.0.0-dev", "0.1.0", false},
		{"", "0.1.0", false},
		{"0.1.0", "f91cdc5", false},
		{"0.1.0", "", false},
	}
	for _, tt := range tests {
		if got := OlderThan(tt.version, tt.other); got != tt.want {
			t.Errorf("OlderThan(%q, %q) = %v, want %v", tt.version, tt.other, got, tt.want)
		}
	}
}

func TestMinFirmwareHasBeenReleased(t *testing.T) {
	text, err := os.ReadFile(changelog)
	if err != nil {
		t.Fatal(err)
	}
	newest, _ := ParseVersion(firstFirmware)
	for _, heading := range regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`).FindAllSubmatch(text, -1) {
		if version, ok := ParseVersion(string(heading[1])); ok && version.Compare(newest) > 0 {
			newest = version
		}
	}

	minimum, ok := ParseVersion(MinFirmware)
	if !ok {
		t.Fatalf("MinFirmware %q is not an X.Y.Z version", MinFirmware)
	}
	if minimum.Compare(newest) > 0 {
		t.Errorf("MinFirmware %s is newer than the newest firmware in CHANGELOG.md (%d.%d.%d): no display could meet it",
			MinFirmware, newest[0], newest[1], newest[2])
	}
}
