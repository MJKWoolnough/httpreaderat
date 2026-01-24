// Package httpreaderat allows opening a URL as a io.ReaderAt.
package httpreaderat

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"vimagination.zapto.org/cache"
)

type block struct {
	data       string
	prev, next *block
}

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// Request represents an io.ReaderAt for an HTTP URL.
type Request struct {
	client    Client
	url       string
	length    int64
	blockSize int64
	cache     *cache.LRU[int64, string]
}

// NewRequest creates a new Request object, for the given URL, that implements
// io.ReaderAt.
//
// Options can be passed in to modify how the maximum length is determined, and
// the characteristics of the block caching.
//
// By default, up to 256 4KB blocks will be cached.
func NewRequest(url string, opts ...Option) (*Request, error) {
	r := &Request{url: url, length: -1, blockSize: 1 << 12}

	for _, opt := range opts {
		opt(r)
	}

	if r.client == nil {
		r.client = http.DefaultClient
	}

	if r.length == -1 {
		if err := r.getLength(); err != nil {
			return nil, err
		}
	}

	if r.cache == nil {
		r.cache = cache.NewLRU[int64, string](256)
	}

	return r, nil
}

func (r *Request) getLength() error {
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}

	resp, err := r.client.Do(req)
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

// ReadAt implements the io.ReaderAt interface.
func (r *Request) ReadAt(p []byte, n int64) (int, error) {
	if r.length >= 0 && n > r.length {
		return 0, io.EOF
	} else if n < 0 {
		return 0, fs.ErrInvalid
	}

	p = p[:min(int64(len(p)), r.length-n)]

	blocks, err := r.getBlocks(n, int64(len(p)))
	if err != nil {
		return 0, err
	}

	blocks[0] = blocks[0][n%r.blockSize:]

	l := len(p)

	for _, block := range blocks {
		p = p[copy(p, block):]
	}

	return l, nil
}

func (r *Request) Length() int64 {
	return r.length
}

func (r *Request) getBlocks(start, length int64) ([]string, error) {
	blocks, requests := r.getExistingBlocks(start, length)
	if len(requests) == 0 {
		return blocks, nil
	}

	if err := r.setNewBlocks(blocks, requests, start); err != nil {
		return nil, err
	}

	return blocks, nil
}

type requests [][2]int64

func (r requests) Iter() iter.Seq[int64] {
	return func(yield func(int64) bool) {
		for _, request := range r {
			for b := request[0]; b <= request[1]; b++ {
				if !yield(b) {
					return
				}
			}
		}
	}
}

func (r *Request) getExistingBlocks(start, length int64) ([]string, requests) {
	var (
		blocks   []string
		requests requests
	)

	startBlock := start / r.blockSize

	for block := startBlock; block <= (start+length-1)/r.blockSize; block++ {
		b, ok := r.cache.Get(block)
		if ok {
			blocks = append(blocks, b)

			continue
		}

		blocks = append(blocks, "")

		if len(requests) > 0 && requests[len(requests)-1][1]+1 == block {
			requests[len(requests)-1][1] = block
		} else {
			requests = append(requests, [2]int64{block, block})
		}
	}

	return blocks, requests
}

func (r *Request) setNewBlocks(blocks []string, requests requests, start int64) error {
	resp, err := r.requestBlocks(requests)
	if err != nil {
		return err
	}

	startBlock := start / r.blockSize
	buf := make([]byte, r.blockSize)

	rr, err := makeReader(resp)
	if err != nil {
		return err
	}

	for block := range requests.Iter() {
		n, err := io.ReadFull(rr, buf[:cmp.Or(min(r.length, (block+1)*r.blockSize)%r.blockSize, r.blockSize)])
		if err != nil {
			return err
		}

		data := string(buf[:n])
		blocks[block-startBlock] = data

		r.cache.Set(block, data)
	}

	return resp.Body.Close()
}

func (r *Request) requestBlocks(requests requests) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Range", r.makeByteRangeHeader(requests))

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (r *Request) makeByteRangeHeader(requests requests) string {
	var byteRange strings.Builder

	byteRange.WriteString("bytes=")

	for n, request := range requests {
		if n > 0 {
			byteRange.WriteByte(',')
		}

		fmt.Fprintf(&byteRange, "%d-%d", request[0]*r.blockSize, min((request[1]+1)*r.blockSize, r.length)-1)
	}

	return byteRange.String()
}

func makeReader(resp *http.Response) (io.Reader, error) {
	var rr io.Reader = resp.Body

	if mt, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil {
		return nil, err
	} else if mt == "multipart/byteranges" {
		mr := multipart.NewReader(resp.Body, params["boundary"])

		p, err := mr.NextPart()
		if err != nil {
			return nil, err
		}

		rr = &multipartReader{mr: mr, p: p}
	}

	return rr, nil
}

type multipartReader struct {
	mr *multipart.Reader
	p  *multipart.Part
}

func (m *multipartReader) Read(p []byte) (int, error) {
	n, err := m.p.Read(p)
	if n == 0 && err == io.EOF {
		var err error

		if m.p, err = m.mr.NextPart(); err != nil {
			return n, err
		}

		return m.Read(p)
	}

	return n, err
}

// Clear clears the block cache.
func (r *Request) Clear() {
	r.cache.Clear()
}

// Errors.
var ErrNoRange = errors.New("no range header")
