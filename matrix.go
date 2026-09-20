package qrkit

import "sync"

// Module kinds in a layout.
const (
	kindData uint8 = iota
	kindFinder
	kindTiming
	kindAlign
	kindFormat // format info, version info and the dark module
)

// layout is the version-specific (but data/level/mask independent) plan of
// a symbol: which modules are function patterns and which codeword bit
// lands in each data module.
type layout struct {
	version int
	size    int
	kind    []uint8
	dark    []bool  // colours of function modules (format bits excluded)
	bitIdx  []int32 // codeword bit index per data module, -1 for none
}

var (
	layoutOnce  [MaxVersion + 1]sync.Once
	layoutCache [MaxVersion + 1]*layout
)

func getLayout(version int) *layout {
	layoutOnce[version].Do(func() { layoutCache[version] = buildLayout(version) })
	return layoutCache[version]
}

func buildLayout(version int) *layout {
	size := symbolSize(version)
	l := &layout{
		version: version,
		size:    size,
		kind:    make([]uint8, size*size),
		dark:    make([]bool, size*size),
		bitIdx:  make([]int32, size*size),
	}
	set := func(x, y int, k uint8, d bool) {
		l.kind[y*size+x] = k
		l.dark[y*size+x] = d
	}
	for i := 0; i < size; i++ {
		set(6, i, kindTiming, i%2 == 0)
		set(i, 6, kindTiming, i%2 == 0)
	}
	for _, c := range [][2]int{{3, 3}, {size - 4, 3}, {3, size - 4}} {
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := c[0]+dx, c[1]+dy
				if x < 0 || y < 0 || x >= size || y >= size {
					continue
				}
				d := maxInt(absInt(dx), absInt(dy))
				set(x, y, kindFinder, d != 2 && d != 4)
			}
		}
	}
	pos := alignmentPositions(version)
	n := len(pos)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if (i == 0 && j == 0) || (i == 0 && j == n-1) || (i == n-1 && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					set(pos[i]+dx, pos[j]+dy, kindAlign, maxInt(absInt(dx), absInt(dy)) != 1)
				}
			}
		}
	}
	fset := func(x, y int, d bool) { set(x, y, kindFormat, d) }
	drawFormatBits(fset, size, LevelLow, 0)
	if version >= 7 {
		drawVersionBits(fset, size, version)
	}

	// Zig-zag data placement.
	for i := range l.bitIdx {
		l.bitIdx[i] = -1
	}
	total := numRawCodewords(version) * 8
	i := 0
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = size - 1 - vert
				}
				if l.kind[y*size+x] == kindData && i < total {
					l.bitIdx[y*size+x] = int32(i)
					i++
				}
			}
		}
	}
	if i != total {
		panic("qrkit: internal error: data placement mismatch")
	}
	return l
}

func formatBits(level RecoveryLevel, mask int) uint32 {
	data := levelFormatBits[level]<<3 | uint32(mask)
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	return (data<<10 | rem) ^ 0x5412
}

func versionBits(version int) uint32 {
	rem := uint32(version)
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	return uint32(version)<<12 | rem
}

func drawFormatBits(set func(x, y int, dark bool), size int, level RecoveryLevel, mask int) {
	bits := formatBits(level, mask)
	bit := func(i int) bool { return bits>>uint(i)&1 == 1 }
	for i := 0; i <= 5; i++ {
		set(8, i, bit(i))
	}
	set(8, 7, bit(6))
	set(8, 8, bit(7))
	set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		set(size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		set(8, size-15+i, bit(i))
	}
	set(8, size-8, true) // the always-dark module
}

func drawVersionBits(set func(x, y int, dark bool), size, version int) {
	bits := versionBits(version)
	for i := 0; i < 18; i++ {
		d := bits>>uint(i)&1 == 1
		a, b := size-11+i%3, i/3
		set(a, b, d)
		set(b, a, d)
	}
}

func maskBit(mask, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	}
	return ((x+y)%2+x*y%3)%2 == 0
}

// buildModules places codewords, chooses (or applies) a mask and draws the
// format information. It returns the final module matrix and the mask used.
func buildModules(l *layout, codewords []byte, level RecoveryLevel, maskOverride int) ([]bool, int) {
	size := l.size
	base := make([]bool, size*size)
	copy(base, l.dark)
	for i, bi := range l.bitIdx {
		if bi >= 0 {
			base[i] = codewords[bi>>3]>>(7-uint(bi&7))&1 == 1
		}
	}
	try := func(mask int) []bool {
		m := make([]bool, len(base))
		copy(m, base)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if l.kind[y*size+x] == kindData && maskBit(mask, x, y) {
					m[y*size+x] = !m[y*size+x]
				}
			}
		}
		drawFormatBits(func(x, y int, d bool) { m[y*size+x] = d }, size, level, mask)
		return m
	}
	if maskOverride >= 0 {
		return try(maskOverride), maskOverride
	}
	var best []bool
	bestMask, bestScore := 0, int(^uint(0)>>1)
	for mask := 0; mask < 8; mask++ {
		m := try(mask)
		if s := penalty(m, size); s < bestScore {
			best, bestMask, bestScore = m, mask, s
		}
	}
	return best, bestMask
}

// penalty scores a masked symbol according to the four ISO 18004 rules.
func penalty(m []bool, size int) int {
	score := 0
	at := func(x, y int) bool { return m[y*size+x] }

	// Rule 1: runs of 5+ same-coloured modules; rule 3: finder-like patterns.
	const p1, p2 = 0x5D0, 0x05D // 10111010000, 00001011101
	for pass := 0; pass < 2; pass++ {
		for a := 0; a < size; a++ {
			run, win := 0, uint32(0)
			var prev bool
			for b := 0; b < size; b++ {
				var cur bool
				if pass == 0 {
					cur = at(b, a)
				} else {
					cur = at(a, b)
				}
				if b > 0 && cur == prev {
					run++
				} else {
					if run >= 5 {
						score += 3 + run - 5
					}
					run = 1
				}
				prev = cur
				win = (win << 1) & 0x7FF
				if cur {
					win |= 1
				}
				if b >= 10 && (win == p1 || win == p2) {
					score += 40
				}
			}
			if run >= 5 {
				score += 3 + run - 5
			}
		}
	}
	// Rule 2: 2x2 blocks.
	dark := 0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := at(x, y)
			if c {
				dark++
			}
			if x < size-1 && y < size-1 && c == at(x+1, y) && c == at(x, y+1) && c == at(x+1, y+1) {
				score += 3
			}
		}
	}
	// Rule 4: dark/light balance.
	total := size * size
	k := (absInt(dark*20-total*10)+total-1)/total - 1
	if k > 0 {
		score += k * 10
	}
	return score
}
