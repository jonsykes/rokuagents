package detect

import (
	"io"
	"os"
)

// ReadTail returns at most the last size bytes of the file at path, and
// whether that is the whole file. Detectors read only the end of a session
// log, however long the session.
func ReadTail(path string, size int64) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	offset := max(info.Size()-size, 0)
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, false, err
	}
	data, err := io.ReadAll(f)
	return data, offset == 0, err
}
