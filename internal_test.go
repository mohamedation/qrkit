package qrkit

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatBits(t *testing.T) {
	// From ISO/IEC 18004 Annex C.
	want := map[RecoveryLevel][8]uint32{
		LevelLow:      {0x77C4, 0x72F3, 0x7DAA, 0x789D, 0x662F, 0x6318, 0x6C41, 0x6976},
		LevelMedium:   {0x5412, 0x5125, 0x5E7C, 0x5B4B, 0x45F9, 0x40CE, 0x4F97, 0x4AA0},
		LevelQuartile: {0x355F, 0x3068, 0x3F31, 0x3A06, 0x24B4, 0x2183, 0x2EDA, 0x2BED},
		LevelHigh:     {0x1689, 0x13BE, 0x1CE7, 0x19D0, 0x0762, 0x0255, 0x0D0C, 0x083B},
	}
	for l, masks := range want {
		for m, w := range masks {
			if got := formatBits(l, m); got != w {
				t.Errorf("formatBits(%v,%d) = %#x, want %#x", l, m, got, w)
			}
		}
	}
}

func TestVersionBits(t *testing.T) {
	// ISO/IEC 18004 Annex D (element i is version i+7).
	want := map[int]uint32{
		7:  0x07C94,
		8:  0x085BC,
		20: 0x149A6,
		40: 0x28C69,
	}
	for v, w := range want {
		if got := versionBits(v); got != w {
			t.Errorf("versionBits(%d) = %#x, want %#x", v, got, w)
		}
	}
}

func TestDataCapacities(t *testing.T) {
	// Data codeword counts from the standard's capacity table.
	cases := []struct {
		v    int
		want [4]int // L, M, Q, H
	}{
		{1, [4]int{19, 16, 13, 9}},
		{5, [4]int{108, 86, 62, 46}},
		{10, [4]int{274, 216, 154, 122}},
		{20, [4]int{861, 669, 485, 385}},
		{32, [4]int{1955, 1541, 1115, 845}},
		{40, [4]int{2956, 2334, 1666, 1276}},
	}
	for _, c := range cases {
		for l := LevelLow; l <= LevelHigh; l++ {
			if got := numDataCodewords(c.v, l); got != c.want[l] {
				t.Errorf("v%d-%v: %d data codewords, want %d", c.v, l, got, c.want[l])
			}
		}
	}
}

func TestAlignmentPositions(t *testing.T) {
	cases := map[int][]int{
		1:  nil,
		2:  {6, 18},
		7:  {6, 22, 38},
		14: {6, 26, 46, 66},
		32: {6, 34, 60, 86, 112, 138},
		36: {6, 24, 50, 76, 102, 128, 154},
		40: {6, 30, 58, 86, 114, 142, 170},
	}
	for v, want := range cases {
		got := alignmentPositions(v)
		if len(got) != len(want) {
			t.Fatalf("v%d: %v want %v", v, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("v%d: %v want %v", v, got, want)
			}
		}
	}
}

func TestLayoutPlacementCoversAllCodewords(t *testing.T) {
	for v := MinVersion; v <= MaxVersion; v++ {
		l := getLayout(v) // panics on mismatch
		n := 0
		for _, b := range l.bitIdx {
			if b >= 0 {
				n++
			}
		}
		if n != numRawCodewords(v)*8 {
			t.Errorf("v%d: %d placed bits, want %d", v, n, numRawCodewords(v)*8)
		}
	}
}

// "HELLO WORLD" at version 1-M: the worked example used throughout the
// QR literature.
func TestHelloWorldCodewords(t *testing.T) {
	data := []byte("HELLO WORLD")
	if chooseMode(data) != modeAlphanumeric {
		t.Fatal("expected alphanumeric mode")
	}
	dcw := encodeDataCodewords(data, modeAlphanumeric, 1, LevelMedium)
	wantData := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	if !bytes.Equal(dcw, wantData) {
		t.Fatalf("data codewords\n got %v\nwant %v", dcw, wantData)
	}
	all := interleaveWithECC(dcw, 1, LevelMedium)
	wantECC := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	if !bytes.Equal(all[16:], wantECC) {
		t.Fatalf("ECC codewords\n got %v\nwant %v", all[16:], wantECC)
	}
}

func TestChooseMode(t *testing.T) {
	for s, want := range map[string]mode{
		"0123456789":     modeNumeric,
		"HELLO $%*+-./:": modeAlphanumeric,
		"hello":          modeByte,
		"héllo":          modeByte,
	} {
		if got := chooseMode([]byte(s)); got != want {
			t.Errorf("chooseMode(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestWriteFileAtomicCleansUpOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	err := writeFileAtomic(path, func(f *os.File) error {
		if _, err := f.WriteString("partial"); err != nil {
			return err
		}
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("destination should not exist after a failed write")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}
