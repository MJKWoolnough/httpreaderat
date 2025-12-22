package httpreaderat

import "vimagination.zapto.org/cache"

// Option is an option that can be parsed to NewRequest.
type Option func(*Request)

// SetLength sets the maximum length of the remote object, bypassing the
// automatic detection.
func SetLength(length int64) Option {
	return func(r *Request) {
		r.length = length
	}
}

// BlockSize changes the store block size from the default 4KB.
func BlockSize(size int64) Option {
	return func(r *Request) {
		r.blockSize = size
	}
}

// CacheCount changes the number of cached blocks from 256.
func CacheCount(count uint64) Option {
	return func(r *Request) {
		r.cache = cache.NewLRU[int64, string](count)
	}
}
