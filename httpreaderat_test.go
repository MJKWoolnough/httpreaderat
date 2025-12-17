package httpreaderat

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed httpreaderat_test.go
var f embed.FS

func TestHTTPReaderAt(t *testing.T) {
	srv := httptest.NewServer(http.FileServerFS(f))

	r, err := NewRequest(srv.URL + "/httpreaderat_test.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	buf := make([]byte, 16)

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
}
