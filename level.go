package qrkit

// RecoveryLevel is the error-correction level of a QR code. Higher levels
// tolerate more damage (or a larger logo) at the cost of a denser symbol.
type RecoveryLevel int

const (
	// LevelLow recovers roughly 7% of the codewords.
	LevelLow RecoveryLevel = iota
	// LevelMedium recovers roughly 15% of the codewords. It is the default.
	LevelMedium
	// LevelQuartile recovers roughly 25% of the codewords.
	LevelQuartile
	// LevelHigh recovers roughly 30% of the codewords. It is the default
	// when a logo is used.
	LevelHigh
)

// String returns "L", "M", "Q" or "H".
func (l RecoveryLevel) String() string {
	switch l {
	case LevelLow:
		return "L"
	case LevelMedium:
		return "M"
	case LevelQuartile:
		return "Q"
	case LevelHigh:
		return "H"
	}
	return "?"
}

func (l RecoveryLevel) valid() bool { return l >= LevelLow && l <= LevelHigh }

// levelFormatBits maps a RecoveryLevel to its 2-bit format indicator.
var levelFormatBits = [4]uint32{1, 0, 3, 2}
