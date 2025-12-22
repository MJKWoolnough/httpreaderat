# httpreaderat

[![CI](https://github.com/MJKWoolnough/httpreaderat/actions/workflows/go-checks.yml/badge.svg)](https://github.com/MJKWoolnough/httpreaderat/actions)
[![Go Reference](https://pkg.go.dev/badge/vimagination.zapto.org/httpreaderat.svg)](https://pkg.go.dev/vimagination.zapto.org/httpreaderat)
[![Go Report Card](https://goreportcard.com/badge/vimagination.zapto.org/httpreaderat)](https://goreportcard.com/report/vimagination.zapto.org/httpreaderat)

--
    import "vimagination.zapto.org/httpreaderat"

Package httpreaderat allows opening a URL as a io.ReaderAt.

## Highlights

 - Simple library to allow random-reading from Range capable HTTP servers.
 - Customisable block-based caching for better performance.

## Usage

```go
package main

import (
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"

	"vimagination.zapto.org/httpreaderat"
)

//go:embed example_test.go
var f embed.FS

func main() {
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
```

## Documentation

Full API docs can be found at:

https://pkg.go.dev/vimagination.zapto.org/httpreaderat
