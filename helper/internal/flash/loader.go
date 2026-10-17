package flash

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// Commands of the ESP32 ROM serial bootloader.
const (
	opSync         = 0x08
	opSpiSetParams = 0x0b
	opReadReg      = 0x0a
	opSpiAttach    = 0x0d
	opChangeBaud   = 0x0f
	opDeflBegin    = 0x10
	opDeflData     = 0x11
	opSpiFlashMD5  = 0x13
)

// Timeouts, as esptool uses them.
const (
	defaultTimeout = 3 * time.Second
	syncTimeout    = 100 * time.Millisecond
	// Flash operations take as long as the amount of data they erase, write or
	// hash, but never less than defaultTimeout.
	erasePerMB = 30 * time.Second
	writePerMB = 40 * time.Second
	md5PerMB   = 8 * time.Second
)

// checksumSeed starts the XOR checksum of a data block.
const checksumSeed = 0xef

// StatusError is an error code in the response of the ROM bootloader.
type StatusError struct {
	Operation string
	Code      uint16 // 0x100 plus the ROM's error byte
}

func (e *StatusError) Error() string {
	reason := romErrors[e.Code]
	if reason == "" {
		reason = "unknown error"
	}
	return fmt.Sprintf("failed to %s: ROM error 0x%x (%s)", e.Operation, e.Code, reason)
}

// romErrors are the codes the ROM bootloader reports.
var romErrors = map[uint16]string{
	0x100: "undefined error",
	0x101: "invalid parameter",
	0x102: "failed to allocate memory",
	0x103: "failed to send the message",
	0x104: "failed to receive the message",
	0x105: "invalid message format",
	0x106: "the command ran but its result is wrong",
	0x107: "checksum error",
	0x108: "flash write error",
	0x109: "flash read error",
	0x10a: "flash read length error",
	0x10b: "deflate error",
	0x10c: "deflate Adler32 error",
	0x10d: "deflate parameter error",
}

// loader sends commands to the ROM bootloader of the chip behind port.
type loader struct {
	ctx    context.Context
	port   Port
	frames frameReader
	sleep  func(time.Duration)
}

func newLoader(ctx context.Context, port Port, sleep func(time.Duration)) *loader {
	return &loader{ctx: ctx, port: port, frames: frameReader{port: port}, sleep: sleep}
}

// response is the answer to a command: the value field of its header and the
// data that follows, status bytes included.
type response struct {
	value uint32
	data  []byte
}

// command sends one command and returns the chip's answer to it. Anything else
// that arrives first (the extra answers to a sync, say) is skipped.
func (l *loader) command(op byte, data []byte, checksum uint32, timeout time.Duration) (response, error) {
	if err := l.ctx.Err(); err != nil {
		return response{}, err
	}
	if err := l.port.SetReadTimeout(timeout); err != nil {
		return response{}, err
	}
	packet := make([]byte, 8, 8+len(data))
	packet[1] = op // packet[0], the direction, is 0 for a request
	binary.LittleEndian.PutUint16(packet[2:], uint16(len(data)))
	binary.LittleEndian.PutUint32(packet[4:], checksum)
	packet = append(packet, data...)
	framed := slipEncode(packet)
	if n, err := l.port.Write(framed); err != nil {
		return response{}, err
	} else if n < len(framed) {
		return response{}, io.ErrShortWrite
	}

	const maxSkipped = 100
	for range maxSkipped {
		frame, err := l.frames.next()
		if err != nil {
			return response{}, err
		}
		if len(frame) < 8 || frame[0] != 1 || frame[1] != op { // not a response to this command
			continue
		}
		return response{value: binary.LittleEndian.Uint32(frame[4:8]), data: frame[8:]}, nil
	}
	return response{}, fmt.Errorf("no response to command 0x%02x among the last %d packets", op, maxSkipped)
}

// check is command for operations that report a status: wantLen bytes of
// result data, then two status bytes (the ESP32 ROM adds two reserved ones).
// The result data is returned in the response.
func (l *loader) check(operation string, op byte, data []byte, checksum uint32, wantLen int, timeout time.Duration) (response, error) {
	r, err := l.command(op, data, checksum, timeout)
	if err != nil {
		return r, fmt.Errorf("%s: %w", operation, err)
	}
	if len(r.data) < wantLen+2 {
		// An error answer has no result data, only the status bytes.
		if len(r.data) >= 2 && r.data[0] != 0 {
			return r, &StatusError{operation, 0x100 | uint16(r.data[1])}
		}
		return r, fmt.Errorf("%s: answer of only %d bytes", operation, len(r.data))
	}
	if status := r.data[wantLen:]; status[0] != 0 {
		return r, &StatusError{operation, 0x100 | uint16(status[1])}
	}
	r.data = r.data[:wantLen]
	return r, nil
}

// sync tells the ROM bootloader, which autodetects the speed, to expect
// commands. It fails if the chip is not in the bootloader.
func (l *loader) sync() error {
	payload := append([]byte{0x07, 0x07, 0x12, 0x20}, bytes.Repeat([]byte{0x55}, 32)...)
	if _, err := l.command(opSync, payload, 0, syncTimeout); err != nil {
		return err
	}
	// The ROM answers a sync eight times. Take the other seven off the line.
	for range 7 {
		if _, err := l.frames.next(); err != nil {
			break
		}
	}
	return nil
}

func (l *loader) readReg(addr uint32) (uint32, error) {
	r, err := l.check("read a register", opReadReg, binary.LittleEndian.AppendUint32(nil, addr), 0, 0, defaultTimeout)
	return r.value, err
}

// changeBaud makes the chip talk at baud. The answer still comes at the old
// speed; the caller changes the port's speed after it.
func (l *loader) changeBaud(baud int) error {
	data := binary.LittleEndian.AppendUint32(nil, uint32(baud))
	data = binary.LittleEndian.AppendUint32(data, 0) // the speed now: only the flasher stub needs it
	_, err := l.command(opChangeBaud, data, 0, defaultTimeout)
	return err
}

// attachFlash connects the SPI flash to the chip's pins as its eFuses
// configure them. The ROM does not do it by itself.
func (l *loader) attachFlash() error {
	const (
		efuseWord3 = 0x3ff5a00c
		efuseWord5 = 0x3ff5a014
	)
	word3, err := l.readReg(efuseWord3)
	if err != nil {
		return err
	}
	word5, err := l.readReg(efuseWord5)
	if err != nil {
		return err
	}
	// The pins of an in-package flash (zero for the usual external one, which
	// takes the default pins) are 5-bit fields of the eFuse words.
	clk, q, d, cs, hd := word5&0x1f, word5>>5&0x1f, word5>>10&0x1f, word5>>15&0x1f, word3>>4&0x1f
	pins := hd<<24 | cs<<18 | d<<12 | q<<6 | clk
	data := binary.LittleEndian.AppendUint32(nil, pins)
	data = append(data, 0, 0, 0, 0) // not the legacy SPI mode
	_, err = l.check("attach the flash", opSpiAttach, data, 0, 0, defaultTimeout)
	return err
}

// setFlashSize tells the ROM how large the flash chip is.
func (l *loader) setFlashSize(size uint32) error {
	var data []byte
	for _, v := range []uint32{
		0,        // flash id
		size,     // total size
		64 << 10, // block size
		4 << 10,  // sector size
		256,      // page size
		0xffff,   // status mask
	} {
		data = binary.LittleEndian.AppendUint32(data, v)
	}
	_, err := l.check("set the flash size", opSpiSetParams, data, 0, 0, defaultTimeout)
	return err
}

// md5 asks the ROM for the MD5 of size bytes of flash at offset, as the 32
// hex digits it returns.
func (l *loader) md5(offset, size uint32) (string, error) {
	var data []byte
	for _, v := range []uint32{offset, size, 0, 0} {
		data = binary.LittleEndian.AppendUint32(data, v)
	}
	r, err := l.check("hash the flash", opSpiFlashMD5, data, 0, 32, timeoutFor(md5PerMB, int(size)))
	return string(r.data), err
}

// timeoutFor is the time to allow for an operation on size bytes at perMB per
// megabyte.
func timeoutFor(perMB time.Duration, size int) time.Duration {
	return max(defaultTimeout, time.Duration(float64(perMB)*float64(size)/1e6))
}

// checksum is the XOR checksum the ROM checks a data block against.
func checksum(block []byte) uint32 {
	sum := byte(checksumSeed)
	for _, b := range block {
		sum ^= b
	}
	return uint32(sum)
}
