package httpreaderat

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

type Request struct {
	url       string
	length    int64
	blockSize int64
	cache     *cache.LRU[int64, string]
}

func NewRequest(url string, opts ...Option) (*Request, error) {
	r := &Request{url: url, length: -1, blockSize: 1 << 12}

	for _, opt := range opts {
		opt(r)
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
	var (
		blocks   []string
		requests [][2]int64
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

	if len(requests) == 0 {
		return blocks, nil
	}

	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}

	var byteRange strings.Builder

	byteRange.WriteString("bytes=")

	for n, request := range requests {
		if n > 0 {
			byteRange.WriteByte(',')
		}

		fmt.Fprintf(&byteRange, "%d-%d", request[0]*r.blockSize, min((request[1]+1)*r.blockSize, r.length)-1)
	}

	req.Header.Set("Range", byteRange.String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, r.blockSize)

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

	for _, request := range requests {
		for b := request[0]; b <= request[1]; b++ {
			n, err := io.ReadFull(rr, buf[:cmp.Or(min(r.length, (b+1)*r.blockSize)%r.blockSize, r.blockSize)])
			if err != nil {
				return nil, err
			}

			data := string(buf[:n])
			blocks[b-startBlock] = data

			r.cache.Set(b, data)
		}
	}

	if err = resp.Body.Close(); err != nil {
		return nil, err
	}

	return blocks, nil
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

var ErrNoRange = errors.New("no range header")
