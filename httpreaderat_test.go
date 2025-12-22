package httpreaderat

import (
	"bytes"
	"crypto/rand"
	"embed"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

//go:embed httpreaderat_test.go
var f embed.FS

func TestHTTPReaderAt(t *testing.T) {
	b, _ := f.ReadFile("httpreaderat_test.go")
	srv := httptest.NewServer(http.FileServerFS(f))

	r, err := NewRequest(srv.URL + "/httpreaderat_test.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	} else if r.Length() != int64(len(b)) {
		t.Errorf("expecting length %d, got %d", len(b), r.Length())
	}

	buf := make([]byte, 16)

	if n, err := r.ReadAt(buf[:12], -1); !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("expecting error ErrInvalid, got %v", err)
	} else if n != 0 {
		t.Errorf("expecting to read %d bytes, read %d", 0, n)
	}

	if n, err := r.ReadAt(buf[:12], 8); err != nil {
		t.Errorf("unexpected error: %s", err)
	} else if n != 12 {
		t.Errorf("expecting to read %d bytes, read %d", 12, n)
	} else if string(buf[:12]) != "httpreaderat" {
		t.Errorf("expecting to read string %q, read %q", "httpreaderat", buf[:12])
	}

	if n, err := r.ReadAt(buf[:7], 0); err != nil {
		t.Errorf("unexpected error: %s", err)
	} else if n != 7 {
		t.Errorf("expecting to read %d bytes, read %d", 7, n)
	} else if string(buf[:7]) != "package" {
		t.Errorf("expecting to read string %q, read %q", "package", buf[:7])
	}

	if n, err := r.ReadAt(buf[:7], 1<<32); !errors.Is(err, io.EOF) {
		t.Errorf("expecting error EOF, got %v", err)
	} else if n != 0 {
		t.Errorf("expecting to read %d bytes, read %d", 0, n)
	}
}

func TestSetLength(t *testing.T) {
	r, err := NewRequest("")
	if err == nil {
		t.Errorf("expecting error, got %v", err)
	} else if r != nil {
		t.Errorf("expecting nil request, got %v", r)
	}

	r, err = NewRequest("", SetLength(10))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	} else if r.Length() != 10 {
		t.Errorf("expecting length %d, got %d", 10, r.Length())
	}
}

type FS []byte

func (f FS) Open(file string) (fs.File, error) {
	return &File{name: file, Reader: bytes.NewReader(f)}, nil
}

type File struct {
	name string
	*bytes.Reader
}

func (File) Close() error                  { return nil }
func (f *File) Stat() (fs.FileInfo, error) { return f, nil }
func (f *File) Name() string               { return f.name }
func (File) IsDir() bool                   { return false }
func (File) ModTime() time.Time            { return time.Now() }
func (File) Mode() fs.FileMode             { return fs.ModePerm }
func (f *File) Sys() any                   { return f }

type requestCounter struct {
	count int
	http.Handler
}

func (rc *requestCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rc.count++

	rc.Handler.ServeHTTP(w, r)
}

func (rc *requestCounter) GetCount() int {
	c := rc.count

	rc.count = 0

	return c
}

func TestLarge(t *testing.T) {
	data := make(FS, 256)

	n, err := rand.Read(data)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	} else if n != len(data) {
		t.Fatalf("only read %d random bytes", n)
	}

	rc := &requestCounter{Handler: http.FileServerFS(data)}

	srv := httptest.NewServer(rc)

	r, err := NewRequest(srv.URL + "/httpreaderat_test.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	} else if r.Length() != int64(len(data)) {
		t.Errorf("expecting length %d, got %d", len(data), r.Length())
	} else if count := rc.GetCount(); count != 1 {
		t.Errorf("expecting 1 request, had %d", count)
	}

	r.blockSize = 10

	testRead(t, data, r, rc, 0, 1, 1)   // 0
	testRead(t, data, r, rc, 0, 1, 0)   // 0
	testRead(t, data, r, rc, 0, 9, 0)   // 0
	testRead(t, data, r, rc, 0, 10, 0)  // 0
	testRead(t, data, r, rc, 1, 10, 1)  // 1,2
	testRead(t, data, r, rc, 11, 10, 1) // 2,3
	testRead(t, data, r, rc, 38, 14, 1) // 3, 4, 5
	testRead(t, data, r, rc, 71, 1, 1)  // 7
	testRead(t, data, r, rc, 68, 14, 1) // 6, 7, 8
}

func testRead(t *testing.T, data FS, r *Request, rc *requestCounter, start, length int64, requests int) {
	t.Helper()

	buf := make([]byte, length)

	if n, err := r.ReadAt(buf, start); err != nil {
		t.Errorf("unexpected error: %s", err)
	} else if int64(n) != length {
		t.Errorf("expected to read %d byte(s), read %d", length, n)
	} else if count := rc.GetCount(); count != requests {
		t.Errorf("expecting %d request(s), had %d", requests, count)
	} else if string(buf) != string(data[start:start+length]) {
		t.Errorf("expected to read %q, read %q", data[start:start+length], buf)
	}
}
