package qrkit

import "errors"

// Sentinel errors returned (possibly wrapped) by this package.
// Use errors.Is to test for them.
var (
	// ErrEmptyData is returned when the content to encode is empty.
	ErrEmptyData = errors.New("qrkit: empty content")

	// ErrDataTooLong is returned when the content does not fit in any
	// permitted symbol version at the requested recovery level.
	ErrDataTooLong = errors.New("qrkit: content too long")

	// ErrInvalidOption is returned when an option has an out-of-range or
	// otherwise invalid value.
	ErrInvalidOption = errors.New("qrkit: invalid option")

	// ErrLogoTooLarge is returned when a logo would cover too much of the
	// symbol to remain reliably scannable (or would cover structural
	// patterns such as the finder patterns).
	ErrLogoTooLarge = errors.New("qrkit: logo too large")
)
