package qrkit

// GF(2^8) arithmetic with the QR primitive polynomial x^8+x^4+x^3+x^2+1.

var gfExp, gfLog = buildGFTables()

func buildGFTables() (exp [512]byte, log [256]int) {
	x := 1
	for i := 0; i < 255; i++ {
		exp[i] = byte(x)
		log[x] = i
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 512; i++ {
		exp[i] = exp[i-255]
	}
	return
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[gfLog[a]+gfLog[b]]
}

// rsDivisor returns the generator polynomial of the given degree, without
// its (implicit) leading 1 coefficient, highest power first.
func rsDivisor(degree int) []byte {
	res := make([]byte, degree)
	res[degree-1] = 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < degree; j++ {
			res[j] = gfMul(res[j], root)
			if j+1 < degree {
				res[j] ^= res[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return res
}

// rsRemainder returns the Reed-Solomon error-correction codewords for data.
func rsRemainder(data, divisor []byte) []byte {
	res := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i, d := range divisor {
			res[i] ^= gfMul(d, factor)
		}
	}
	return res
}
