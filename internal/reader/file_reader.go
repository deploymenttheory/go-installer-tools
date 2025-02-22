package reader

import (
	"bytes"
	"io"
	"os"
)

// TempFileReader wraps an io.Reader and provides the ability to rewind
// by buffering content as it is read.
type TempFileReader struct {
	Reader io.Reader
	buf    *bytes.Buffer
	tmpBuf []byte
}

// Read implements io.Reader interface.
func (t *TempFileReader) Read(p []byte) (n int, err error) {
	if t.buf == nil {
		t.buf = &bytes.Buffer{}
	}
	if t.tmpBuf == nil {
		t.tmpBuf = make([]byte, 32*1024) // 32KB buffer
	}

	n, err = t.Reader.Read(p)
	if n > 0 {
		// Write the data to our buffer
		if n2, err2 := t.buf.Write(p[:n]); err2 != nil {
			return n2, err2
		}
	}
	return n, err
}

// Rewind resets the reader to the beginning of the buffered content.
func (t *TempFileReader) Rewind() error {
	if t.buf == nil {
		t.buf = &bytes.Buffer{}
	}

	// If the original reader is a file, try to rewind it
	if f, ok := t.Reader.(*os.File); ok {
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
	}

	// Create a new buffer with the existing content
	newBuf := bytes.NewBuffer(t.buf.Bytes())
	t.buf = newBuf
	return nil
}

// ReadAt implements io.ReaderAt interface
func (t *TempFileReader) ReadAt(p []byte, off int64) (n int, err error) {
	if t.buf == nil {
		// If we haven't read anything yet, we need to read the entire file
		t.buf = &bytes.Buffer{}
		if _, err := io.Copy(t.buf, t.Reader); err != nil {
			return 0, err
		}
	}

	// Read from our buffer at the specified offset
	return bytes.NewReader(t.buf.Bytes()).ReadAt(p, off)
}

// Size returns the total size of the buffered content
func (t *TempFileReader) Size() int64 {
	if t.buf == nil {
		return 0
	}
	return int64(t.buf.Len())
}
