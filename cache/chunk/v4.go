package chunk

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"unsafe"

	lz4 "github.com/DataDog/golz4"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

const (
	blockSize          = 1024 * 1024 // 1 MB
	blockCompressBound = blockSize + ((blockSize) / 255) + 16
)

var (
	blockBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, blockSize)
		},
	}

	blockCompressionBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, blockCompressBound)
		},
	}
)

func getBlockBuffer() []byte {
	return blockBufferPool.Get().([]byte)[:0]
}

func putBlockBuffer(buf []byte) {
	blockBufferPool.Put(buf)
}

func getCompressionBuffer() []byte {
	return blockCompressionBufferPool.Get().([]byte)[:0]
}

func putCompressionBuffer(buf []byte) {
	blockCompressionBufferPool.Put(buf)
}

func writeBlocks(w io.Writer, from timeseries.Time, step timeseries.Duration, pointsCount int, values []*model.MetricValues) error {
	data := getBlockBuffer()
	defer putBlockBuffer(data)
	compressionBuf := getCompressionBuffer()[:blockCompressBound]
	defer putCompressionBuffer(compressionBuf)
	floatBuf := make([]float32, pointsCount)
	nans := make([]float32, pointsCount)
	valueSize := pointsCount * 4
	to := from.Add(timeseries.Duration(pointsCount-1) * step)

	for i := range nans {
		nans[i] = timeseries.NaN
	}

	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		n, err := lz4.Compress(compressionBuf, data)
		if err != nil {
			return err
		}
		m := blockMeta{
			Decompressed: uint32(len(data)),
			Compressed:   uint32(n),
		}
		if err = binary.Write(w, binary.LittleEndian, &m); err != nil {
			return err
		}
		_, err = w.Write(compressionBuf[:n])
		data = data[:0]
		return err
	}

	for _, mv := range values {
		if len(data)+valueSize > blockSize {
			if err := flush(); err != nil {
				return err
			}
		}
		copy(floatBuf, nans)
		iter := mv.Values.Iter()
		for iter.Next() {
			t, v := iter.Value()
			if t > to {
				break
			}
			if t < from {
				continue
			}
			floatBuf[int((t-from)/timeseries.Time(step))] = v
		}
		data = append(data, asBytes32(floatBuf)...)

	}
	return flush()
}

type blockReader struct {
	r              io.Reader
	compressionBuf []byte
	data           []byte

	h         *header
	offset    int
	valueSize int
}

type blockMeta struct {
	Decompressed uint32
	Compressed   uint32
}

func (br *blockReader) readNextBlock() error {
	var m blockMeta
	if err := binary.Read(br.r, binary.LittleEndian, &m); err != nil {
		return err
	}
	br.compressionBuf = br.compressionBuf[:m.Compressed]
	if _, err := io.ReadFull(br.r, br.compressionBuf); err != nil {
		return err
	}
	_, err := lz4.Uncompress(br.data, br.compressionBuf)
	if err != nil {
		return err
	}
	br.data = br.data[:m.Decompressed]
	br.offset = 0
	return nil
}

func newBlockReader(r io.Reader, h *header) (*blockReader, error) {
	br := &blockReader{
		r:              r,
		data:           getBlockBuffer()[:blockSize],
		compressionBuf: getCompressionBuffer(),
		valueSize:      int(h.PointsCount) * 4,
		h:              h,
	}
	if err := br.readNextBlock(); err != nil {
		return nil, err
	}
	return br, nil
}

func (br *blockReader) reclaimBuffers() {
	putBlockBuffer(br.data)
	putCompressionBuffer(br.compressionBuf)
}

func (br *blockReader) read(mv *model.MetricValues, fillFunc timeseries.FillFunc) (bool, error) {
	if br.offset+br.valueSize > blockSize {
		if err := br.readNextBlock(); err != nil {
			return false, err
		}
	}
	changed := fillFunc(mv.Values, br.h.From, br.h.Step, asFloats32(br.data[br.offset:br.offset+br.valueSize]))
	br.offset += br.valueSize
	return changed, nil
}

func Write(f io.Writer, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []*model.MetricValues) (err error) {
	w := bufio.NewWriter(f)
	h := &header{
		Version:                V4,
		From:                   from,
		PointsCount:            uint32(pointsCount),
		Step:                   step,
		Finalized:              finalized,
		DataSizeOrMetricsCount: uint32(len(metrics)),
	}
	if err = binary.Write(w, binary.LittleEndian, h); err != nil {
		return err
	}
	hashes := make([]uint64, 0, len(metrics))
	for _, mv := range metrics {
		hashes = append(hashes, mv.LabelsHash)
	}
	if _, err = w.Write(asBytes64(hashes)); err != nil {
		return err
	}
	if err = writeBlocks(w, from, step, pointsCount, metrics); err != nil {
		return err
	}
	if err = writeLabelsV4(w, metrics); err != nil {
		return err
	}
	return w.Flush()
}

func readNStrings(buf []byte, n int) ([]string, error) {
	res := make([]string, 0, n)
	offset := 0
	size := 0
	for i := 0; i < n; i++ {
		if offset+2 > len(buf) {
			return nil, io.ErrUnexpectedEOF
		}
		size = int(binary.LittleEndian.Uint16(buf[offset:]))
		offset += 2
		if size == 0 { // TODO: remove
			size = math.MaxUint16 + 1
		}
		if offset+size > len(buf) {
			return nil, io.ErrUnexpectedEOF
		}
		res = append(res, unsafe.String(&buf[offset], size))
		offset += size
	}
	return res, nil
}

type labelsMeta struct {
	CompressedValsSize uint32
	KeysSize           uint32
	ValsSize           uint32
	KeysDictSize       uint32
	ValsDictSize       uint32
	PairsSize          uint32
}

type dict struct {
	d   map[string]int
	buf *bytes.Buffer
}

func newDict() *dict {
	return &dict{
		d:   make(map[string]int),
		buf: bytes.NewBuffer(nil),
	}
}

func (d *dict) idx(s string) int {
	idx, ok := d.d[s]
	if !ok {
		idx = len(d.d)
		d.d[s] = idx
		size := len(s)
		if size > math.MaxUint16 {
			size = math.MaxUint16
		}
		_ = binary.Write(d.buf, binary.LittleEndian, uint16(size))
		_, _ = d.buf.Write(unsafe.Slice(unsafe.StringData(s), size))
	}
	return idx
}

func pairsCount(mv *model.MetricValues) int {
	s := 0
	if mv.MachineID != "" {
		s++
	}
	if mv.SystemUUID != "" {
		s++
	}
	if mv.ContainerId != "" {
		s++
	}
	if mv.Destination != "" {
		s++
	}
	if mv.ActualDestination != "" {
		s++
	}
	for _, v := range mv.Labels {
		if v != "" {
			s++
		}
	}
	return s
}

func writeLabelsV4(w *bufio.Writer, metrics []*model.MetricValues) error {
	keys := newDict()
	vals := newDict()
	labelPairs := bytes.NewBuffer(nil)
	var pairBuf [4]byte

	write := func(k, v string) error {
		if k == "" {
			return errors.New("empty label key")
		}
		if v == "" {
			return nil
		}
		ki := keys.idx(k)
		vi := vals.idx(v)
		if vi >= (1 << 24) {
			return errors.New("valIdx value exceeds 24-bit limit")
		}
		pairBuf[0] = byte(ki)
		pairBuf[1] = byte(vi)
		pairBuf[2] = byte(vi >> 8)
		pairBuf[3] = byte(vi >> 16)
		_, err := labelPairs.Write(pairBuf[:])
		return err
	}
	var err error
	for _, mv := range metrics {
		if err = labelPairs.WriteByte(byte(pairsCount(mv))); err != nil {
			return err
		}
		if mv.MachineID != "" {
			if err = write(model.LabelMachineId, mv.MachineID); err != nil {
				return err
			}
		}
		if mv.SystemUUID != "" {
			if err = write(model.LabelSystemUuid, mv.SystemUUID); err != nil {
				return err
			}
		}
		if mv.ContainerId != "" {
			if err = write(model.LabelContainerId, mv.ContainerId); err != nil {
				return err
			}
		}
		if mv.Destination != "" {
			label := model.LabelDestination
			if mv.DestIp {
				label = model.LabelDestinationIP
			}
			if err = write(label, mv.Destination); err != nil {
				return err
			}
		}
		if mv.ActualDestination != "" {
			if err = write(model.LabelActualDestination, mv.ActualDestination); err != nil {
				return err
			}
		}
		for k, v := range mv.Labels {
			if err = write(k, v); err != nil {
				return err
			}
		}
	}

	compressedVals := make([]byte, lz4.CompressBound(vals.buf.Bytes()))
	n, err := lz4.Compress(compressedVals, vals.buf.Bytes())
	if err != nil {
		return err
	}
	lm := &labelsMeta{
		KeysSize:           uint32(keys.buf.Len()),
		ValsSize:           uint32(vals.buf.Len()),
		KeysDictSize:       uint32(len(keys.d)),
		ValsDictSize:       uint32(len(vals.d)),
		PairsSize:          uint32(labelPairs.Len()),
		CompressedValsSize: uint32(n),
	}

	if err = binary.Write(w, binary.LittleEndian, lm); err != nil {
		return err
	}
	if _, err = w.Write(keys.buf.Bytes()); err != nil {
		return err
	}
	if _, err = w.Write(compressedVals[:n]); err != nil {
		return err
	}
	if _, err = w.Write(labelPairs.Bytes()); err != nil {
		return err
	}
	return nil
}

func readLabelsV4(r *bufio.Reader, hashes []uint64, missing map[uint64]*model.MetricValues) error {
	if len(missing) == 0 {
		return nil
	}
	lm := &labelsMeta{}
	if err := binary.Read(r, binary.LittleEndian, lm); err != nil {
		return fmt.Errorf("failed to read meta: %w", err)
	}
	keysData := make([]byte, lm.KeysSize)
	if _, err := io.ReadFull(r, keysData); err != nil {
		return fmt.Errorf("failed to read keys data: %w", err)
	}
	keys, err := readNStrings(keysData, int(lm.KeysDictSize))
	if err != nil {
		return fmt.Errorf("failed to read keys: %w", err)
	}
	compressedVals := make([]byte, int(lm.CompressedValsSize))
	uncompressedVals := make([]byte, int(lm.ValsSize))

	if _, err = io.ReadFull(r, compressedVals); err != nil {
		return fmt.Errorf("failed to read compressed vals: %w", err)
	}
	if _, err = lz4.Uncompress(uncompressedVals, compressedVals); err != nil {
		return fmt.Errorf("failed to uncompress vals: %w", err)
	}
	vals, err := readNStrings(uncompressedVals, int(lm.ValsDictSize))
	if err != nil {
		return fmt.Errorf("failed to read vals: %w", err)
	}
	var ki, vi int
	var pairsNum byte
	var key, val string
	var idx [4]byte
	for _, h := range hashes {
		pairsNum, err = r.ReadByte()
		if err != nil {
			return err
		}
		mv := missing[h]
		if mv == nil {
			if _, err = r.Discard(int(pairsNum * 4)); err != nil {
				return err
			}
			continue
		}
		mv.Labels = make(model.Labels)
		for i := 0; i < int(pairsNum); i++ {
			if _, err = io.ReadFull(r, idx[:]); err != nil {
				return err
			}
			ki = int(idx[0])
			vi = int(idx[1]) | int(idx[2])<<8 | int(idx[3])<<16

			if ki >= len(keys) || vi >= len(vals) {
				return errors.New("corrupted labels buffer: key or value index out of range")
			}
			key = keys[ki]
			val = vals[vi]
			switch key {
			case model.LabelMachineId:
				mv.MachineID = val
			case model.LabelSystemUuid:
				mv.SystemUUID = val
			case model.LabelContainerId:
				mv.ContainerId = val
			case model.LabelDestination:
				mv.Destination = val
			case model.LabelActualDestination:
				mv.ActualDestination = val
			case model.LabelDestinationIP:
				mv.ActualDestination = val
				mv.DestIp = true
			default:
				mv.Labels[key] = val
			}
		}
	}
	return nil
}

func readV4(reader *bufio.Reader, h *header, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]*model.MetricValues, fillFunc timeseries.FillFunc) error {
	var err error
	hashesBuf := make([]byte, int(h.DataSizeOrMetricsCount)*8)
	if _, err = io.ReadFull(reader, hashesBuf); err != nil {
		return err
	}
	hashes := asUint64(hashesBuf)
	br, err := newBlockReader(reader, h)
	if err != nil {
		return err
	}
	defer br.reclaimBuffers()
	missing := map[uint64]*model.MetricValues{}
	for _, hash := range hashes {
		mv, exists := dest[hash]
		if mv == nil {
			mv = &model.MetricValues{
				LabelsHash: hash,
				Values:     timeseries.New(from, pointsCount, step),
			}
			missing[hash] = mv
		}
		changed, err := br.read(mv, fillFunc)
		if err != nil {
			return fmt.Errorf("failed to read data: %w", err)
		}
		if changed && !exists {
			dest[hash] = mv
		}
	}
	if err = readLabelsV4(reader, hashes, missing); err != nil {
		return fmt.Errorf("failed to read labels: %w", err)
	}
	return nil
}
