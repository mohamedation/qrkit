package qrkit

type mode uint8

const (
	modeNumeric mode = iota
	modeAlphanumeric
	modeByte
)

const alnumCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

var alnumTable = func() (t [256]int8) {
	for i := range t {
		t[i] = -1
	}
	for i := 0; i < len(alnumCharset); i++ {
		t[alnumCharset[i]] = int8(i)
	}
	return
}()

func (m mode) indicator() uint32 {
	switch m {
	case modeNumeric:
		return 0x1
	case modeAlphanumeric:
		return 0x2
	}
	return 0x4
}

// charCountBits returns the width of the character-count field.
func charCountBits(m mode, version int) int {
	var w [3]int
	switch m {
	case modeNumeric:
		w = [3]int{10, 12, 14}
	case modeAlphanumeric:
		w = [3]int{9, 11, 13}
	default:
		w = [3]int{8, 16, 16}
	}
	switch {
	case version <= 9:
		return w[0]
	case version <= 26:
		return w[1]
	}
	return w[2]
}

// chooseMode picks the most compact single mode able to represent data.
func chooseMode(data []byte) mode {
	numeric := true
	for _, b := range data {
		if alnumTable[b] < 0 {
			return modeByte
		}
		if b < '0' || b > '9' {
			numeric = false
		}
	}
	if numeric {
		return modeNumeric
	}
	return modeAlphanumeric
}

func payloadBits(m mode, n int) int {
	switch m {
	case modeNumeric:
		return 10*(n/3) + [3]int{0, 4, 7}[n%3]
	case modeAlphanumeric:
		return 11*(n/2) + 6*(n%2)
	}
	return 8 * n
}

// totalBits is the bit length of the segment (indicator+count+payload).
func totalBits(m mode, n, version int) int {
	return 4 + charCountBits(m, version) + payloadBits(m, n)
}

type bitBuffer struct {
	b []byte
	n int
}

func (bb *bitBuffer) append(v uint32, count int) {
	for i := count - 1; i >= 0; i-- {
		if bb.n%8 == 0 {
			bb.b = append(bb.b, 0)
		}
		bb.b[bb.n/8] |= byte(v>>uint(i)&1) << (7 - uint(bb.n%8))
		bb.n++
	}
}

// encodeDataCodewords builds the padded data codeword sequence.
func encodeDataCodewords(data []byte, m mode, version int, level RecoveryLevel) []byte {
	var bb bitBuffer
	bb.append(m.indicator(), 4)
	bb.append(uint32(len(data)), charCountBits(m, version))
	switch m {
	case modeNumeric:
		for i := 0; i < len(data); i += 3 {
			end := i + 3
			if end > len(data) {
				end = len(data)
			}
			v := uint32(0)
			for _, c := range data[i:end] {
				v = v*10 + uint32(c-'0')
			}
			bb.append(v, [4]int{0, 4, 7, 10}[end-i])
		}
	case modeAlphanumeric:
		for i := 0; i < len(data); i += 2 {
			if i+1 < len(data) {
				bb.append(uint32(alnumTable[data[i]])*45+uint32(alnumTable[data[i+1]]), 11)
			} else {
				bb.append(uint32(alnumTable[data[i]]), 6)
			}
		}
	default:
		for _, c := range data {
			bb.append(uint32(c), 8)
		}
	}
	capBits := numDataCodewords(version, level) * 8
	term := capBits - bb.n
	if term > 4 {
		term = 4
	}
	bb.append(0, term)
	if r := bb.n % 8; r != 0 {
		bb.append(0, 8-r)
	}
	for pad := uint32(0xEC); bb.n < capBits; pad ^= 0xEC ^ 0x11 {
		bb.append(pad, 8)
	}
	return bb.b
}

// blockInfo describes the error-correction block structure of a symbol.
type blockInfo struct {
	numBlocks int
	eccLen    int
	numShort  int // blocks with one fewer data codeword
	shortData int // data codewords in a short block
	rawCW     int
}

func newBlockInfo(version int, l RecoveryLevel) blockInfo {
	nb := numBlocks[l][version]
	ecc := eccCodewordsPerBlock[l][version]
	raw := numRawCodewords(version)
	return blockInfo{
		numBlocks: nb,
		eccLen:    ecc,
		numShort:  nb - raw%nb,
		shortData: raw/nb - ecc,
		rawCW:     raw,
	}
}

func (b blockInfo) dataLen(block int) int {
	if block < b.numShort {
		return b.shortData
	}
	return b.shortData + 1
}

// order returns, for every position of the final interleaved codeword
// stream, the block it comes from and its index within that block's
// (data || ecc) sequence.
func (b blockInfo) order() (blocks, index []int) {
	blocks = make([]int, 0, b.rawCW)
	index = make([]int, 0, b.rawCW)
	for i := 0; i <= b.shortData; i++ {
		for j := 0; j < b.numBlocks; j++ {
			if i < b.dataLen(j) {
				blocks = append(blocks, j)
				index = append(index, i)
			}
		}
	}
	for i := 0; i < b.eccLen; i++ {
		for j := 0; j < b.numBlocks; j++ {
			blocks = append(blocks, j)
			index = append(index, b.dataLen(j)+i)
		}
	}
	return
}

// interleaveWithECC splits data into blocks, appends Reed-Solomon codewords
// and returns the interleaved stream.
func interleaveWithECC(data []byte, version int, l RecoveryLevel) []byte {
	bi := newBlockInfo(version, l)
	div := rsDivisor(bi.eccLen)
	seqs := make([][]byte, bi.numBlocks)
	k := 0
	for j := range seqs {
		n := bi.dataLen(j)
		d := data[k : k+n]
		k += n
		seqs[j] = append(append(make([]byte, 0, n+bi.eccLen), d...), rsRemainder(d, div)...)
	}
	blocks, index := bi.order()
	out := make([]byte, len(blocks))
	for i := range out {
		out[i] = seqs[blocks[i]][index[i]]
	}
	return out
}
