package httpreaderat

import "vimagination.zapto.org/cache"

type Option func(*Request)

func SetLength(length int64) Option {
	return func(r *Request) {
		r.length = length
	}
}

func BlockSize(size int64) Option {
	return func(r *Request) {
		r.blockSize = size
	}
}

func CacheCount(count uint64) Option {
	return func(r *Request) {
		r.cache = cache.NewLRU[int64, string](count)
	}
}
