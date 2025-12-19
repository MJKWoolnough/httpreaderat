package httpreaderat

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
)

type Request struct {
	url    string
	length int64
}

func NewRequest(url string, opts ...Option) (*Request, error) {
	r := &Request{url: url, length: -1}

	for _, opt := range opts {
		opt(r)
	}

	if r.length == -1 {
		if err := r.getLength(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *Request) getLength() error {
	resp, err := http.Head(r.url)
	if err != nil {
		return err
	}

	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return ErrNoRange
	}

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if r.length, err = strconv.ParseInt(cl, 10, 64); err != nil {
			return fmt.Errorf("error parsing content-length: %w", err)
		}
	}

	return nil
}

func (r *Request) ReadAt(p []byte, n int64) (int, error) {
	if r.length >= 0 && n > r.length {
		return 0, io.EOF
	} else if n < 0 {
		return 0, fs.ErrInvalid
	}

	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", n, n+int64(len(p))))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	return resp.Body.Read(p)
}

func (r *Request) Length() int64 {
	return r.length
}

var ErrNoRange = errors.New("no range header")
