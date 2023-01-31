package pool

import (
	"sync"
)

var (
	b1k   = &sync.Pool{}
	b10k  = &sync.Pool{}
	b100k = &sync.Pool{}
	b1M   = &sync.Pool{}
	b10M  = &sync.Pool{}
	b100M = &sync.Pool{}
)

func GetByteArray(l int) []byte {
	var p *sync.Pool
	var capacity int
	switch {
	case l < 1024:
		p = b1k
		capacity = 1024
	case l < 10*1024:
		p = b10k
		capacity = 10 * 1024
	case l < 100*1024:
		p = b100k
		capacity = 100 * 1024
	case l < 1024*1024:
		p = b1M
		capacity = 1024 * 1024
	case l < 10*1024*1024:
		p = b10M
		capacity = 10 * 1024 * 1024
	case l < 100*1024*1024:
		p = b100M
		capacity = 100 * 1024 * 1024
	default:
		return make([]byte, l)
	}
	buf, ok := p.Get().([]byte)
	if !ok {
		buf = make([]byte, 0, capacity)
	}
	return buf[:l]
}

func PutByteArray(buf []byte) {
	l := cap(buf)
	var p *sync.Pool
	switch l {
	case 1024:
		p = b1k
	case 10 * 1024:
		p = b10k
	case 100 * 1024:
		p = b100k
	case 1024 * 1024:
		p = b1M
	case 10 * 1024 * 1024:
		p = b10M
	case 100 * 1024 * 1024:
		p = b100M
	default:
		return
	}
	p.Put(buf)
}
