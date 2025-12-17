package httpreaderat

import (
	"errors"
	"fmt"
	"net/http"
)

type Request struct {
	url string
}

func NewRequest(url string) (*Request, error) {
	resp, err := http.Head(url)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, ErrNoRange
	}

	return &Request{url: url}, nil
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

var ErrNoRange = errors.New("no range header")
