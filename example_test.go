package httpreaderat_test

import (
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"

	"vimagination.zapto.org/httpreaderat"
)

//go:embed example_test.go
var f embed.FS

func Example() {
	srv := httptest.NewServer(http.FileServerFS(f))

	r, err := httpreaderat.NewRequest(srv.URL + "/example_test.go")
	if err != nil {
		fmt.Println(err)

		return
	}

	buf := make([]byte, 17)

	n, err := r.ReadAt(buf, 8)
	fmt.Printf("Bytes: %d\nErr: %v\nRead: %s", n, err, buf)

	// Output:
	// Bytes: 17
	// Err: <nil>
	// Read: httpreaderat_test
}
