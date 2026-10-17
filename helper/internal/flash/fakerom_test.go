package flash

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"io"
	"testing"
	"time"
)

// fakeROM is an in-memory ESP32 with its ROM bootloader behind a Port. It
// follows the protocol strictly (SLIP framing, command checksums, block
// sequence numbers, attach before flash commands) so that a mistake in the
// flasher shows as an error here, as it would on a chip.
type fakeROM struct {
	t *testing.T

	// Behaviour to provoke.
	magic         uint32         // value of the chip magic register
	ignoreSyncs   int            // syncs the chip does not answer, as when it is still booting
	silent        bool           // the chip never answers
	ignoreBaud    bool           // the chip ignores CHANGE_BAUDRATE
	bridgeMaxBaud int            // the USB bridge garbles anything faster; 0 for no limit
	failOn        map[byte]uint8 // ROM error byte to answer a command with
	failDataOnce  map[int]uint8  // ROM error byte for the first try of the data block with this sequence number
	corruptMD5    bool

	// State of the line and the chip.
	dtr, rts   bool
	bootloader bool // the chip runs the ROM bootloader
	attached   bool
	sized      bool
	portBaud   int
	chipBaud   int
	in         []byte // bytes the host wrote, not decoded yet
	out        []byte // bytes the chip sent, not read yet

	// What the chip saw.
	flash      []byte
	resets     int  // resets into the bootloader
	hardReset  bool // the chip was reset into the firmware
	baudChange int  // speed the chip was told to change to
	commands   []byte
	begin      struct{ size, blocks, blockSize, offset uint32 }
	stream     []byte // compressed data received
	nextSeq    uint32
	failed     map[int]bool
}

func newFakeROM(t *testing.T) *fakeROM {
	f := &fakeROM{
		t:            t,
		magic:        esp32Magic,
		failOn:       map[byte]uint8{},
		failDataOnce: map[int]uint8{},
		failed:       map[int]bool{},
		portBaud:     RomBaud,
		chipBaud:     RomBaud,
		flash:        bytes.Repeat([]byte{0xff}, FlashSize),
	}
	return f
}

func (f *fakeROM) Read(p []byte) (int, error) {
	n := copy(p, f.out)
	f.out = f.out[n:]
	return n, nil // nothing to read is a timeout
}

func (f *fakeROM) Write(p []byte) (int, error) {
	f.in = append(f.in, p...)
	f.decode()
	return len(p), nil
}

func (f *fakeROM) SetReadTimeout(time.Duration) error { return nil }
func (f *fakeROM) Close() error                       { return nil }
func (f *fakeROM) ResetInputBuffer() error            { f.out = nil; return nil }
func (f *fakeROM) SetBaudRate(baud int) error         { f.portBaud = baud; return nil }
func (f *fakeROM) SetDTR(v bool) error                { f.setLines(v, f.rts); return nil }
func (f *fakeROM) SetRTS(v bool) error                { f.setLines(f.dtr, v); return nil }

// setLines follows the auto-reset circuit: the chip resets while RTS is
// asserted, and starts the bootloader if DTR holds GPIO0 low as RTS lets go.
func (f *fakeROM) setLines(dtr, rts bool) {
	wasReset := f.rts
	f.dtr, f.rts = dtr, rts
	if wasReset && !rts {
		f.chipBaud = RomBaud
		f.in = nil
		f.attached, f.sized = false, false
		f.bootloader = dtr
		if dtr {
			f.resets++
			f.out = append(f.out, "ets Jun  8 2016 00:22:57\r\n\r\nrst:0x1 (POWERON_RESET),boot:0x3 (DOWNLOAD_BOOT(UART0/UART1/SDIO_REI_REO_V2))\r\nwaiting for download\r\n"...)
		} else {
			f.hardReset = true
		}
	}
}

// decode handles the complete SLIP frames in f.in.
func (f *fakeROM) decode() {
	for {
		start := bytes.IndexByte(f.in, slipEnd)
		if start < 0 {
			return
		}
		end := bytes.IndexByte(f.in[start+1:], slipEnd)
		if end < 0 {
			return
		}
		raw := f.in[start+1 : start+1+end]
		f.in = f.in[start+1+end+1:]
		if len(raw) == 0 {
			continue
		}
		var frame []byte
		for i := 0; i < len(raw); i++ {
			if raw[i] != slipEsc {
				frame = append(frame, raw[i])
				continue
			}
			i++
			switch {
			case i < len(raw) && raw[i] == slipEscEnd:
				frame = append(frame, slipEnd)
			case i < len(raw) && raw[i] == slipEscEsc:
				frame = append(frame, slipEsc)
			default:
				f.t.Errorf("invalid SLIP escape in a packet from the host")
			}
		}
		f.handle(frame)
	}
}

func (f *fakeROM) handle(frame []byte) {
	if f.silent || !f.bootloader || f.portBaud != f.chipBaud ||
		(f.bridgeMaxBaud != 0 && f.portBaud > f.bridgeMaxBaud) {
		return // nobody listens, or the line is garbled
	}
	if len(frame) < 8 || frame[0] != 0 {
		f.t.Errorf("malformed request % x", frame)
		return
	}
	op, size, chk := frame[1], int(binary.LittleEndian.Uint16(frame[2:])), binary.LittleEndian.Uint32(frame[4:])
	data := frame[8:]
	if len(data) != size {
		f.t.Errorf("command 0x%02x: header says %d data bytes, packet has %d", op, size, len(data))
		return
	}
	f.commands = append(f.commands, op)

	if code, ok := f.failOn[op]; ok {
		f.reply(op, 0, []byte{1, code, 0, 0})
		return
	}
	switch op {
	case opSync:
		if f.ignoreSyncs > 0 {
			f.ignoreSyncs--
			return
		}
		if !bytes.Equal(data, append([]byte{0x07, 0x07, 0x12, 0x20}, bytes.Repeat([]byte{0x55}, 32)...)) {
			f.t.Errorf("sync payload % x", data)
		}
		for range 8 {
			f.reply(op, 0x07071220, []byte{0, 0, 0, 0})
		}
	case opReadReg:
		val := uint32(0)
		if addr := binary.LittleEndian.Uint32(data); addr == chipMagicReg {
			val = f.magic
		}
		f.reply(op, val, []byte{0, 0, 0, 0})
	case opChangeBaud:
		if f.ignoreBaud {
			return
		}
		f.baudChange = int(binary.LittleEndian.Uint32(data))
		f.reply(op, 0, []byte{0, 0, 0, 0})
		f.chipBaud = f.baudChange // the answer left at the old speed
	case opSpiAttach:
		if len(data) != 8 {
			f.t.Errorf("SPI_ATTACH takes 8 bytes, got %d", len(data))
		}
		f.attached = true
		f.reply(op, 0, []byte{0, 0, 0, 0})
	case opSpiSetParams:
		if len(data) != 24 || binary.LittleEndian.Uint32(data[4:]) != FlashSize {
			f.t.Errorf("SPI_SET_PARAMS % x", data)
		}
		f.sized = true
		f.reply(op, 0, []byte{0, 0, 0, 0})
	case opDeflBegin:
		f.deflBegin(data)
	case opDeflData:
		f.deflData(data, chk)
	case opSpiFlashMD5:
		offset, n := binary.LittleEndian.Uint32(data), binary.LittleEndian.Uint32(data[4:])
		sum := md5.Sum(f.flash[offset : offset+n])
		hexSum := hex.EncodeToString(sum[:])
		if f.corruptMD5 {
			hexSum = "0" + hexSum[1:]
			if hexSum == hex.EncodeToString(sum[:]) {
				hexSum = "1" + hexSum[1:]
			}
		}
		f.reply(op, 0, append([]byte(hexSum), 0, 0, 0, 0))
	default:
		f.t.Errorf("unexpected command 0x%02x", op)
	}
}

func (f *fakeROM) deflBegin(data []byte) {
	if len(data) != 16 {
		f.t.Errorf("DEFL_BEGIN takes 16 bytes on the ESP32 ROM, got %d", len(data))
		return
	}
	if !f.attached || !f.sized {
		f.reply(opDeflBegin, 0, []byte{1, 0x05, 0, 0}) // invalid message: flash not attached
		return
	}
	f.begin.size = binary.LittleEndian.Uint32(data)
	f.begin.blocks = binary.LittleEndian.Uint32(data[4:])
	f.begin.blockSize = binary.LittleEndian.Uint32(data[8:])
	f.begin.offset = binary.LittleEndian.Uint32(data[12:])
	f.stream, f.nextSeq = nil, 0
	// The ROM erases what it was told to before it answers.
	region := f.flash[f.begin.offset : f.begin.offset+f.begin.size]
	for i := range region {
		region[i] = 0xff
	}
	f.reply(opDeflBegin, 0, []byte{0, 0, 0, 0})
}

func (f *fakeROM) deflData(data []byte, chk uint32) {
	if len(data) < 16 {
		f.t.Errorf("DEFL_DATA of only %d bytes", len(data))
		return
	}
	n, seq := binary.LittleEndian.Uint32(data), binary.LittleEndian.Uint32(data[4:])
	block := data[16:]
	switch {
	case int(n) != len(block):
		f.t.Errorf("block %d: header says %d bytes, has %d", seq, n, len(block))
	case len(block) > int(f.begin.blockSize):
		f.t.Errorf("block %d: %d bytes, more than the block size", seq, len(block))
	case seq != f.nextSeq:
		f.t.Errorf("block %d where %d was expected", seq, f.nextSeq)
	}
	if code, ok := f.failDataOnce[int(seq)]; ok && !f.failed[int(seq)] {
		f.failed[int(seq)] = true
		f.reply(opDeflData, 0, []byte{1, code, 0, 0})
		return
	}
	sum := byte(0xef)
	for _, b := range block {
		sum ^= b
	}
	if uint32(sum) != chk {
		f.reply(opDeflData, 0, []byte{1, 0x07, 0, 0}) // checksum error
		return
	}
	f.stream = append(f.stream, block...)
	f.nextSeq++
	if f.nextSeq == f.begin.blocks {
		f.inflate()
	}
	f.reply(opDeflData, 0, []byte{0, 0, 0, 0})
}

// inflate writes the decompressed stream to flash, as the ROM does while the
// blocks arrive.
func (f *fakeROM) inflate() {
	r, err := zlib.NewReader(bytes.NewReader(f.stream))
	if err != nil {
		f.t.Errorf("the stream is not zlib: %v", err)
		return
	}
	image, err := io.ReadAll(r) // also checks the Adler-32
	if err != nil {
		f.t.Errorf("inflating: %v", err)
	}
	if uint32(len(image)) > f.begin.size {
		f.t.Errorf("%d bytes inflated into a region of %d", len(image), f.begin.size)
		return
	}
	copy(f.flash[f.begin.offset:], image)
}

// reply queues a response frame.
func (f *fakeROM) reply(op byte, value uint32, data []byte) {
	packet := []byte{1, op, 0, 0}
	binary.LittleEndian.PutUint16(packet[2:], uint16(len(data)))
	packet = binary.LittleEndian.AppendUint32(packet, value)
	f.out = append(f.out, slipEncode(append(packet, data...))...)
}
