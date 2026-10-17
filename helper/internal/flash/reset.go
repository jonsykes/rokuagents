package flash

import (
	"errors"
	"fmt"
	"time"
)

// On the board the USB bridge's DTR and RTS lines drive the chip through two
// transistors: RTS asserted holds the chip in reset (EN low), DTR asserted
// pulls GPIO0 low. A chip that leaves reset with GPIO0 low starts the ROM
// bootloader instead of the firmware.

// Reset timing, as in esptool's classic reset sequence.
const (
	resetHold       = 100 * time.Millisecond // the chip is held in reset this long
	bootDelay       = 50 * time.Millisecond  // GPIO0 stays low this long after reset
	slowBootDelay   = 550 * time.Millisecond // the retry, for a slow bridge or a big capacitor on EN
	connectAttempts = 7                      // resets, alternating the two boot delays
	syncAttempts    = 5                      // syncs after each reset
	syncRetryWait   = 50 * time.Millisecond
)

// ErrNoBootloader means the chip did not start its ROM bootloader.
var ErrNoBootloader = errors.New("could not start the chip's bootloader")

// lines is the state of the two modem lines, held for hold.
type lines struct {
	dtr, rts bool
	hold     time.Duration
}

// setLines walks through states, setting DTR before RTS as the reset
// sequences require.
func (l *loader) setLines(states ...lines) error {
	for _, s := range states {
		if err := l.port.SetDTR(s.dtr); err != nil {
			return err
		}
		if err := l.port.SetRTS(s.rts); err != nil {
			return err
		}
		// The Windows driver of some bridges only sends the new RTS state
		// along with a DTR update.
		if err := l.port.SetDTR(s.dtr); err != nil {
			return err
		}
		l.sleep(s.hold)
	}
	return nil
}

// resetIntoBootloader resets the chip with GPIO0 held low.
func (l *loader) resetIntoBootloader(delay time.Duration) error {
	return l.setLines(
		lines{dtr: false, rts: true, hold: resetHold}, // reset, GPIO0 high
		lines{dtr: true, rts: false, hold: delay},     // out of reset with GPIO0 low
		lines{dtr: false, rts: false},                 // GPIO0 high again
	)
}

// resetIntoFirmware resets the chip with GPIO0 high, which runs the flash.
func (l *loader) resetIntoFirmware() error {
	return l.setLines(lines{rts: true, hold: resetHold}, lines{})
}

// connect resets the chip into its bootloader and syncs with it, trying
// again with the longer boot delay if that fails.
func (l *loader) connect() error {
	var lastErr error
	for attempt := range connectAttempts {
		delay := bootDelay
		if attempt%2 == 1 {
			delay = slowBootDelay
		}
		if err := l.port.ResetInputBuffer(); err != nil {
			return err
		}
		l.frames.discard()
		if err := l.resetIntoBootloader(delay); err != nil {
			return fmt.Errorf("resetting the chip: %w", err)
		}
		for range syncAttempts {
			if err := l.ctx.Err(); err != nil {
				return err
			}
			if err := l.port.ResetInputBuffer(); err != nil { // the ROM's boot log
				return err
			}
			l.frames.discard()
			if lastErr = l.sync(); lastErr == nil {
				return nil
			}
			l.sleep(syncRetryWait)
		}
	}
	return fmt.Errorf("%w (%v): is the board connected by a data cable and not in use by another program?",
		ErrNoBootloader, lastErr)
}
