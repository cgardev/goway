package goway

import (
	"hash/crc32"
	"testing"
)

// byteOrderMark is the UTF-8 BOM constructed from raw bytes so that this source
// file itself does not begin with, or contain, a stray marker.
var byteOrderMark = string([]byte{0xEF, 0xBB, 0xBF})

// crcOf returns the signed CRC32 checksum of the given bytes, matching how
// calculateChecksum interprets the result.
func crcOf(content string) int32 {
	return int32(crc32.ChecksumIEEE([]byte(content)))
}

func TestCalculateChecksumEmptyIsZero(t *testing.T) {
	if got := calculateChecksum(nil); got != 0 {
		t.Fatalf("checksum of empty content = %d, want 0", got)
	}
	if got := calculateChecksum([]byte("")); got != 0 {
		t.Fatalf("checksum of empty string = %d, want 0", got)
	}
}

func TestCalculateChecksumIgnoresLineTerminators(t *testing.T) {
	// The checksum is the CRC32 of every line concatenated without terminators,
	// so newlines must not influence the value.
	want := crcOf("abc")
	for _, content := range []string{"abc", "a\nb\nc", "a\r\nb\r\nc", "a\rb\rc"} {
		if got := calculateChecksum([]byte(content)); got != want {
			t.Errorf("checksum(%q) = %d, want %d", content, got, want)
		}
	}
}

func TestCalculateChecksumTrailingNewlineDoesNotMatter(t *testing.T) {
	if calculateChecksum([]byte("a\nb")) != calculateChecksum([]byte("a\nb\n")) {
		t.Error("a trailing newline changed the checksum")
	}
}

func TestCalculateChecksumStripsByteOrderMarkFromFirstLineOnly(t *testing.T) {
	if calculateChecksum([]byte(byteOrderMark+"abc")) != crcOf("abc") {
		t.Error("byte order marker was not stripped from the first line")
	}

	// On any line other than the first, the marker bytes are part of the content.
	got := calculateChecksum([]byte(byteOrderMark + "a\n" + byteOrderMark + "b"))
	want := crcOf("a" + byteOrderMark + "b")
	if got != want {
		t.Errorf("checksum with marker on second line = %d, want %d", got, want)
	}
}

func TestCalculateChecksumIsStable(t *testing.T) {
	script := "CREATE TABLE users (\n    id INTEGER PRIMARY KEY\n);\n"
	if calculateChecksum([]byte(script)) != calculateChecksum([]byte(script)) {
		t.Fatal("checksum is not deterministic")
	}
}
