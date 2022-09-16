package chunk

import (
	"encoding/binary"
	pool "github.com/libp2p/go-buffer-pool"
	"github.com/pierrec/lz4"
)

func compress(src []byte) ([]byte, error) {
	l := lz4.CompressBlockBound(len(src))
	dst := pool.Get(l + 4)
	binary.LittleEndian.PutUint32(dst, uint32(len(src)))
	n, err := lz4.CompressBlock(src, dst[4:], nil)
	if err != nil {
		pool.Put(dst)
	}
	return dst[:n+4], err
}

func decompress(src []byte) ([]byte, error) {
	l := binary.LittleEndian.Uint32(src)
	if l == 0 {
		return nil, nil
	}
	dst := pool.Get(int(l))
	n, err := lz4.UncompressBlock(src[4:], dst)
	if err != nil {
		pool.Put(dst)
	}
	return dst[:n], err
}
