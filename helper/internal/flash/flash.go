// Package flash writes firmware to an ESP32 over its serial port, using the
// chip's built-in ROM bootloader. Nothing is installed on the chip: the ROM
// cannot be overwritten, so a failed or interrupted flash can always be redone.
//
// The protocol is the one esptool speaks to a chip without a flasher stub;
// esptool is the reference for every constant here. The stub would write
// faster, but it is a binary blob per chip that the helper would have to carry.
package flash

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	// RomBaud is the speed the ROM bootloader starts at.
	RomBaud = 115200
	// DefaultBaud is the faster speed Write switches to. The CH340C bridge of
	// the display keeps up with 460800; 921600 fails.
	DefaultBaud = 460800
	// FlashSize is the size of the flash chip of the supported boards.
	FlashSize = 4 << 20

	sectorSize = 4 << 10
	blockSize  = 0x400 // the ROM takes the compressed data in blocks of this size
	// writeAttempts is how often a data block is sent before the flash fails:
	// a lost byte on the line shows as a checksum error.
	writeAttempts = 3

	// chipMagicReg holds a value that identifies the chip family.
	chipMagicReg = 0x40001000
	esp32Magic   = 0x00f01d83
)

var (
	// ErrWrongChip means the chip behind the port is not a classic ESP32.
	ErrWrongChip = errors.New("not an ESP32 chip")
	// ErrVerify means the flash does not hold the image that was written.
	ErrVerify = errors.New("the flash does not match the image")
)

// Phase is a step of Write.
type Phase int

const (
	Connecting Phase = iota // resetting the chip into its bootloader
	Erasing                 // the ROM erases the whole region before it takes data
	Writing
	Verifying
	Resetting // starting the new firmware
)

func (p Phase) String() string {
	return [...]string{"connecting", "erasing", "writing", "verifying", "resetting"}[p]
}

// Progress reports a step of Write. During Writing, Done of Total bytes of the
// compressed image have been written.
type Progress struct {
	Phase       Phase
	Done, Total int
}

// Options are the optional settings of Write.
type Options struct {
	// Baud is the speed for the transfer. Zero means DefaultBaud; RomBaud
	// keeps the ROM's speed.
	Baud int
	// Progress is called at the start of each phase and after each block
	// written (Done is then 0 at the start of Writing). It may be nil.
	Progress func(Progress)

	sleep func(time.Duration) // time.Sleep, replaced by tests
}

// Result describes a completed Write.
type Result struct {
	Baud       int // the speed the data went at: Options.Baud, or RomBaud if the port did not manage it
	Compressed int // the bytes sent, after compression
}

// Write flashes image at offset on the chip behind port, checks it with the
// chip's MD5 and restarts the chip so that it runs the new firmware. It
// resets the chip into its bootloader first; port is left open and at the
// speed in Result. The caller owns the port.
//
// image is padded to a multiple of four bytes, as the ROM requires. The
// region it covers is erased before anything is written, so after an error or
// a cancelled context the old firmware is gone and Write has to be run again.
func Write(ctx context.Context, port Port, image []byte, offset uint32, opts Options) (Result, error) {
	if len(image) == 0 {
		return Result{}, errors.New("the firmware image is empty")
	}
	image = padTo4(image)
	switch {
	case offset%sectorSize != 0:
		return Result{}, fmt.Errorf("flash offset 0x%x is not a multiple of the %d byte sector size", offset, sectorSize)
	case uint64(offset)+uint64(len(image)) > FlashSize:
		return Result{}, fmt.Errorf("a %d byte image at 0x%x does not fit the %d byte flash", len(image), offset, FlashSize)
	}
	if opts.Baud == 0 {
		opts.Baud = DefaultBaud
	}
	if opts.sleep == nil {
		opts.sleep = time.Sleep
	}
	report := func(p Progress) {
		if opts.Progress != nil {
			opts.Progress(p)
		}
	}
	l := newLoader(ctx, port, opts.sleep)

	report(Progress{Phase: Connecting})
	if err := l.connect(); err != nil {
		return Result{}, err
	}
	if err := l.checkChip(); err != nil {
		return Result{}, err
	}
	baud, err := l.raiseBaud(opts.Baud)
	if err != nil {
		return Result{}, err
	}
	if err := l.attachFlash(); err != nil {
		return Result{}, err
	}
	if err := l.setFlashSize(FlashSize); err != nil {
		return Result{}, err
	}

	compressed, err := compress(image)
	if err != nil {
		return Result{}, err
	}
	if err := l.writeCompressed(image, compressed, offset, report); err != nil {
		return Result{}, err
	}

	report(Progress{Phase: Verifying})
	want := md5.Sum(image)
	got, err := l.md5(offset, uint32(len(image)))
	if err != nil {
		return Result{}, err
	}
	if got != hex.EncodeToString(want[:]) {
		return Result{}, fmt.Errorf("%w: MD5 %s, expected %x", ErrVerify, got, want)
	}

	report(Progress{Phase: Resetting})
	if err := l.resetIntoFirmware(); err != nil {
		return Result{}, fmt.Errorf("restarting the chip: %w", err)
	}
	return Result{Baud: baud, Compressed: len(compressed)}, nil
}

// padTo4 pads image with erased flash bytes to a multiple of four.
func padTo4(image []byte) []byte {
	if pad := (4 - len(image)%4) % 4; pad != 0 {
		return append(image[:len(image):len(image)], bytes.Repeat([]byte{0xff}, pad)...)
	}
	return image
}

// compress deflates image as the zlib stream the ROM inflates.
func compress(image []byte) ([]byte, error) {
	var out bytes.Buffer
	w, err := zlib.NewWriterLevel(&out, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(image); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// checkChip refuses anything but a classic ESP32.
func (l *loader) checkChip() error {
	magic, err := l.readReg(chipMagicReg)
	if err != nil {
		return err
	}
	if magic != esp32Magic {
		return fmt.Errorf("%w (chip magic 0x%08x): managents displays have a classic ESP32", ErrWrongChip, magic)
	}
	return nil
}

// raiseBaud switches the chip and the port to baud and returns the speed in
// use. If the chip or the USB bridge does not follow, it goes back to the ROM's
// speed, which needs a new reset, and returns that.
func (l *loader) raiseBaud(baud int) (int, error) {
	if baud <= RomBaud {
		return RomBaud, nil
	}
	err := l.switchBaud(baud)
	if err == nil {
		return baud, nil
	}
	if l.ctx.Err() != nil {
		return 0, l.ctx.Err()
	}
	if err := l.port.SetBaudRate(RomBaud); err != nil {
		return 0, err
	}
	if err := l.connect(); err != nil {
		return 0, err
	}
	return RomBaud, nil
}

// switchBaud tells the chip to change its speed, follows it, and checks that
// they understand each other.
func (l *loader) switchBaud(baud int) error {
	if err := l.changeBaud(baud); err != nil {
		return err
	}
	if err := l.port.SetBaudRate(baud); err != nil {
		return err
	}
	l.sleep(50 * time.Millisecond) // the chip sends garbage while it changes speed
	if err := l.port.ResetInputBuffer(); err != nil {
		return err
	}
	l.frames.discard()
	return l.checkChip() // a command at the new speed proves both ends agree
}

// writeCompressed sends compressed, the zlib stream of image, to be inflated
// and written at offset.
func (l *loader) writeCompressed(image, compressed []byte, offset uint32, report func(Progress)) error {
	// The ROM wants the size rounded up to whole blocks, and erases that much
	// before it answers.
	eraseSize := (len(image) + blockSize - 1) / blockSize * blockSize
	blocks := (len(compressed) + blockSize - 1) / blockSize
	var begin []byte
	for _, v := range []uint32{uint32(eraseSize), uint32(blocks), blockSize, offset} {
		begin = binary.LittleEndian.AppendUint32(begin, v)
	}
	report(Progress{Phase: Erasing})
	if _, err := l.check("erase the flash", opDeflBegin, begin, 0, 0, timeoutFor(erasePerMB, eraseSize)); err != nil {
		return err
	}

	report(Progress{Phase: Writing, Total: len(compressed)})
	for seq := range blocks {
		start := seq * blockSize
		block := compressed[start:min(start+blockSize, len(compressed))]
		// The ROM writes a block's data before it answers. How much that is
		// depends on how well the block compressed; take the average.
		written := int(uint64(len(image)) * uint64(len(block)) / uint64(len(compressed)))
		data := make([]byte, 0, 16+len(block))
		for _, v := range []uint32{uint32(len(block)), uint32(seq), 0, 0} {
			data = binary.LittleEndian.AppendUint32(data, v)
		}
		data = append(data, block...)
		var err error
		for range writeAttempts {
			_, err = l.check("write to the flash", opDeflData, data, checksum(block), 0, timeoutFor(writePerMB, written))
			if err == nil || l.ctx.Err() != nil {
				break
			}
		}
		if err != nil {
			return err
		}
		report(Progress{Phase: Writing, Done: start + len(block), Total: len(compressed)})
	}
	return nil
}
