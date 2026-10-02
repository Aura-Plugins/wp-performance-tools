package archive

/*
An All-in-One WP Migration .wpress file is a flat list of entries:

	[ header 4377 bytes ][ file contents ][ header ][ contents ] … [ end block 4377 bytes ]

Header layout: name (255) · size (14) · mtime (12) · path prefix (4096), each padded with
NUL bytes (older versions pad with spaces). The archive must end with an end block: all
zeros in older versions, or an empty name plus the archive's content size in newer ones.
AI1WM reports "The archive file appears to be corrupted" when it is missing — typically
an interrupted download or cloud sync.
*/

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

const (
	headerSize = 4377
	nameLen    = 255
	sizeLen    = 14
	mtimeLen   = 12
)

// Report describes an archive's structure and the manifest (package.json) inside it.
type Report struct {
	Path      string         `json:"path"`
	SizeBytes int64          `json:"size_bytes"`
	Entries   int            `json:"entries"`
	Complete  bool           `json:"complete"`
	Problem   string         `json:"problem,omitempty"`
	Encrypted bool           `json:"encrypted"`
	Manifest  map[string]any `json:"manifest,omitempty"` // package.json, minus secrets
}

// Check walks the archive header by header without extracting anything.
func Check(path string) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	rep := &Report{Path: path, SizeBytes: st.Size()}

	header := make([]byte, headerSize)
	zero := make([]byte, headerSize)
	var pos int64

	for {
		n, err := io.ReadFull(f, header)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			rep.Problem = fmt.Sprintf("truncated: header at byte %d is only %d of %d bytes and the end-of-archive block is missing", pos, n, headerSize)
			return rep, nil
		}
		if err != nil {
			return nil, err
		}
		if bytes.Equal(header, zero) {
			rep.Complete = true
			return rep, nil
		}

		name := field(header[:nameLen])
		sizeField := field(header[nameLen : nameLen+sizeLen])

		// Newer AI1WM versions write the end block with an empty name and the archive size
		// (everything before the end block) in the size field, as an integrity check.
		if name == "" {
			if sizeField == "" || sizeField == strconv.FormatInt(pos, 10) {
				rep.Complete = true
			} else {
				rep.Problem = fmt.Sprintf("end-of-archive block records %s bytes of content but the archive has %d", sizeField, pos)
			}
			return rep, nil
		}

		size, err := strconv.ParseInt(sizeField, 10, 64)
		if err != nil {
			rep.Problem = fmt.Sprintf("invalid header at byte %d (not a .wpress file, or corrupted)", pos)
			return rep, nil
		}
		contentStart := pos + headerSize
		if contentStart+size > rep.SizeBytes {
			rep.Problem = fmt.Sprintf("truncated: entry %q (%d bytes) runs past the end of the file", name, size)
			return rep, nil
		}

		if name == "package.json" && rep.Manifest == nil {
			if err := readManifest(f, size, rep); err != nil {
				return nil, err
			}
		} else if _, err := f.Seek(size, io.SeekCurrent); err != nil {
			return nil, err
		}

		rep.Entries++
		pos = contentStart + size
	}
}

func readManifest(f *os.File, size int64, rep *Report) error {
	buf := make([]byte, size)
	if _, err := io.ReadFull(f, buf); err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil // unreadable manifest is not fatal; the structure check still stands
	}
	if _, ok := m["EncryptedSignature"]; ok {
		rep.Encrypted = true
	}
	for _, k := range []string{"EncryptedSignature", "Server", "SecretKey"} {
		delete(m, k) // never print secrets or server config
	}
	rep.Manifest = m
	return nil
}

// field trims NUL and space padding from a header field.
func field(b []byte) string {
	return string(bytes.Trim(b, "\x00 "))
}
