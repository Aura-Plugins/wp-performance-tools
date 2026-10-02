package archive

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// header builds a 4377-byte .wpress entry header.
func header(name string, size int64) []byte {
	h := make([]byte, headerSize)
	copy(h, name)
	copy(h[nameLen:], fmt.Sprint(size))
	copy(h[nameLen+sizeLen:], "1700000000")
	copy(h[nameLen+sizeLen+mtimeLen:], ".")
	return h
}

func writeArchive(t *testing.T, parts ...[]byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.wpress")
	if err := os.WriteFile(p, bytes.Join(parts, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

var manifest = []byte(`{"SiteURL":"https://example.test","EncryptedSignature":"secret","Server":{".htaccess":"x"}}`)

func TestCheckCompleteZeroEndBlock(t *testing.T) {
	p := writeArchive(t,
		header("package.json", int64(len(manifest))), manifest,
		header("a.txt", 5), []byte("hello"),
		make([]byte, headerSize),
	)
	rep, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Complete || rep.Entries != 2 {
		t.Fatalf("want complete with 2 entries, got complete=%v entries=%d problem=%q", rep.Complete, rep.Entries, rep.Problem)
	}
	if !rep.Encrypted {
		t.Error("want Encrypted from EncryptedSignature")
	}
	if _, ok := rep.Manifest["EncryptedSignature"]; ok {
		t.Error("secret field must be stripped from the manifest")
	}
	if _, ok := rep.Manifest["Server"]; ok {
		t.Error("server config must be stripped from the manifest")
	}
}

func TestCheckCompleteSizedEndBlock(t *testing.T) {
	content := append(header("a.txt", 5), []byte("hello")...)
	p := writeArchive(t, content, header("", int64(len(content))))
	rep, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Complete {
		t.Fatalf("newer end block (empty name + archive size) must be accepted, problem=%q", rep.Problem)
	}
}

func TestCheckSizedEndBlockMismatch(t *testing.T) {
	content := append(header("a.txt", 5), []byte("hello")...)
	p := writeArchive(t, content, header("", 999))
	rep, _ := Check(p)
	if rep.Complete {
		t.Fatal("end block recording the wrong size must not count as complete")
	}
}

func TestCheckTruncatedHeader(t *testing.T) {
	full := header("b.jpg", 3)
	p := writeArchive(t, header("a.txt", 5), []byte("hello"), full[:2929])
	rep, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Complete || rep.Entries != 1 {
		t.Fatalf("want incomplete after 1 entry, got complete=%v entries=%d", rep.Complete, rep.Entries)
	}
	if len(Findings(rep)) == 0 || Findings(rep)[0].ID != "archive.corrupted" {
		t.Error("want an archive.corrupted finding")
	}
}

func TestCheckTruncatedContent(t *testing.T) {
	p := writeArchive(t, header("a.txt", 500), []byte("short"))
	rep, _ := Check(p)
	if rep.Complete {
		t.Fatal("entry running past end of file must be incomplete")
	}
}
