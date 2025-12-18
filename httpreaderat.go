package httpreaderat

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type Request struct {
	url    string
	length int64
}

func NewRequest(url string) (*Request, error) {
	resp, err := http.Head(url)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, ErrNoRange
	}

	contentLength := int64(-1)

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if contentLength, err = strconv.ParseInt(cl, 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing content-length: %w", err)
		}
	}

	return &Request{url: url, length: contentLength}, nil
}

func (r *Request) ReadAt(p []byte, n int64) (int, error) {
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
