package httpreaderat

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
