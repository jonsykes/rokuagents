package flash

import (
	"errors"
	"fmt"
)

// The ROM bootloader frames every packet with SLIP: it starts and ends with
// slipEnd, and slipEnd and slipEsc inside it are replaced by two-byte
// escape sequences.
const (
	slipEnd    = 0xc0
	slipEsc    = 0xdb
	slipEscEnd = 0xdc // follows slipEsc for a slipEnd in the data
	slipEscEsc = 0xdd // follows slipEsc for a slipEsc in the data

	maxFrame = 1 << 16 // frames are about 1 KiB; more means line noise
)

// ErrNoResponse means the chip did not answer within the read timeout.
var ErrNoResponse = errors.New("no response from the chip")

// slipEncode frames packet.
func slipEncode(packet []byte) []byte {
	out := make([]byte, 0, len(packet)+16)
	out = append(out, slipEnd)
	for _, b := range packet {
		switch b {
		case slipEnd:
			out = append(out, slipEsc, slipEscEnd)
		case slipEsc:
			out = append(out, slipEsc, slipEscEsc)
		default:
			out = append(out, b)
		}
	}
	return append(out, slipEnd)
}

// frameReader decodes the SLIP frames the chip sends. It skips what comes
// before a frame: after a reset the ROM prints its boot log in text.
type frameReader struct {
	port    Port
	buf     [1024]byte
	pending []byte // received, not decoded yet
	frame   []byte // the frame being decoded
	inFrame bool
	escaped bool
}

// next returns the next complete frame, or ErrNoResponse if the port stays
// silent for its read timeout.
func (r *frameReader) next() ([]byte, error) {
	for {
		for len(r.pending) > 0 {
			b := r.pending[0]
			r.pending = r.pending[1:]
			switch {
			case !r.inFrame:
				r.inFrame = b == slipEnd
			case r.escaped:
				r.escaped = false
				switch b {
				case slipEscEnd:
					r.frame = append(r.frame, slipEnd)
				case slipEscEsc:
					r.frame = append(r.frame, slipEsc)
				default:
					r.discard()
					return nil, fmt.Errorf("invalid SLIP escape 0x%02x: line noise?", b)
				}
			case b == slipEsc:
				r.escaped = true
			case b == slipEnd && len(r.frame) == 0:
				// Still the start: frames may be delimited by two ends.
			case b == slipEnd:
				frame := r.frame
				r.frame, r.inFrame = nil, false
				return frame, nil
			default:
				r.frame = append(r.frame, b)
				if len(r.frame) > maxFrame {
					r.discard()
					return nil, errors.New("SLIP frame without an end: line noise?")
				}
			}
		}
		n, err := r.port.Read(r.buf[:])
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, ErrNoResponse
		}
		r.pending = r.buf[:n]
	}
}

// discard forgets everything received so far, the frame in progress included.
func (r *frameReader) discard() {
	r.pending, r.frame, r.inFrame, r.escaped = nil, nil, false, false
}
