package chunk

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"

	lz4 "github.com/DataDog/golz4"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

const (
	blockSize          = 1024 * 1024 // 1 MB
	blockCompressBound = blockSize + ((blockSize) / 255) + 16
	DefaultMetricName  = "metric"
)

var (
	invalidFields = errors.New("invalid fields")

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

type groupChunkHeader struct {
	MetricsInGroup  uint8
	MetricNamesSize uint32
}

type GroupMetadata struct {
	Hash       uint64
	LabelsSize uint32
}

func writeBlocks(w io.Writer, metricsCount int, from timeseries.Time, step timeseries.Duration, pointsCount int, groups []*model.MetricValues) error {
	data := getBlockBuffer()
	defer putBlockBuffer(data)
	compressionBuf := getCompressionBuffer()[:blockCompressBound]
	defer putCompressionBuffer(compressionBuf)
	floatBuf := make([]float32, pointsCount)
	nans := make([]float32, pointsCount)
	groupSize := metricsCount * pointsCount * 4
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

	for _, group := range groups {
		if len(data)+groupSize > blockSize {
			if err := flush(); err != nil {
				return err
			}
		}
		for _, ts := range group.Values {
			copy(floatBuf, nans)
			iter := ts.Iter()
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
	}
	return flush()
}

type blockReader struct {
	r              io.Reader
	compressionBuf []byte
	data           []byte

	h              *header
	groupValueSize int
	offset         int
	metricsMapping []int
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

func newBlockReader(r io.Reader, chunkMetricNames, resultMetrics []string, h *header) (*blockReader, error) {
	br := &blockReader{
		r:              r,
		data:           getBlockBuffer()[:blockSize],
		compressionBuf: getCompressionBuffer(),
		groupValueSize: len(chunkMetricNames) * int(h.PointsCount) * 4,
		h:              h,
		metricsMapping: make([]int, len(chunkMetricNames)),
	}
	for i, name := range chunkMetricNames {
		idx := -1
		for j, resName := range resultMetrics {
			if resName == name {
				idx = j
				break
			}
		}
		br.metricsMapping[i] = idx
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
	if br.offset+br.groupValueSize > blockSize {
		if err := br.readNextBlock(); err != nil {
			return false, err
		}
	}
	changed := false
	for _, idx := range br.metricsMapping {
		if idx >= 0 {
			changed = fillFunc(mv.Values[idx], br.h.From, br.h.Step, asFloats32(br.data[br.offset:br.offset+int(br.h.PointsCount)*4]))
		}
		br.offset += int(br.h.PointsCount) * 4
	}
	return changed, nil
}

func writeGroupsLabels(w io.Writer, groups []*model.MetricValues) error {
	z := lz4.NewWriter(w)
	zw := bufio.NewWriterSize(z, 16*1024)
	sizes := make([]uint32, 0, len(groups))
	for _, group := range groups {
		sizes = append(sizes, uint32(metadataSize(group)))
	}
	if _, err := zw.Write(asBytes32(sizes)); err != nil {
		return err
	}
	for _, group := range groups {
		if err := writeLabels(zw, group); err != nil {
			return err
		}
	}
	if err := zw.Flush(); err != nil {
		return err
	}
	return z.Close()
}

func Write(f io.Writer, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, groups []*model.MetricValues, metrics []string) (err error) {
	w := bufio.NewWriter(f)
	metricNames := strings.Join(metrics, "\x00")
	metricsCount := len(metrics)
	h := &header{
		Version:                V4,
		From:                   from,
		PointsCount:            uint32(pointsCount),
		Step:                   step,
		Finalized:              finalized,
		DataSizeOrMetricsCount: uint32(len(groups)),
	}
	if err = binary.Write(w, binary.LittleEndian, h); err != nil {
		return err
	}
	gh := groupChunkHeader{
		MetricsInGroup:  uint8(metricsCount),
		MetricNamesSize: uint32(len(metricNames)),
	}
	if err = binary.Write(w, binary.LittleEndian, gh); err != nil {
		return err
	}
	if _, err = w.WriteString(metricNames); err != nil {
		return err
	}
	hashes := make([]uint64, 0, len(groups))
	for _, group := range groups {
		hashes = append(hashes, group.LabelsHash)
	}
	if _, err = w.Write(asBytes64(hashes)); err != nil {
		return err
	}
	if err = writeBlocks(w, metricsCount, from, step, pointsCount, groups); err != nil {
		return err
	}
	if err = writeGroupsLabels(w, groups); err != nil {
		return err
	}
	return w.Flush()
}

func readMetricNames(r io.Reader, gh *groupChunkHeader) ([]string, error) {
	buf := make([]byte, gh.MetricNamesSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	res := make([]string, 0, gh.MetricsInGroup)
	for _, f := range bytes.Split(buf, []byte{0}) {
		res = append(res, string(f))
	}
	if len(res) != int(gh.MetricsInGroup) {
		return nil, invalidFields
	}
	return res, nil
}

func readGroupLabels(r io.Reader, hashes []uint64, missingLabels []int, dest map[uint64]*model.MetricValues) error {
	if len(missingLabels) == 0 {
		return nil
	}
	labelsSizedBuf := make([]byte, len(hashes)*4)
	zr := lz4.NewDecompressReader(r)
	defer zr.Close()

	if _, err := io.ReadFull(zr, labelsSizedBuf); err != nil {
		panic(err)
		return err
	}
	labelsSizes := asUint32(labelsSizedBuf)
	offsets := make([]uint32, 0, len(hashes))
	var maxSize uint32
	offset := uint32(0)
	for _, s := range labelsSizes {
		offsets = append(offsets, offset)
		offset += s
		if s > maxSize {
			maxSize = s
		}
	}
	buf := make([]byte, maxSize)
	offset = 0
	for _, idx := range missingLabels {
		h := hashes[idx]
		s := labelsSizes[idx]
		o := offsets[idx]
		mv, ok := dest[h]
		if !ok {
			continue
		}
		toSkip := o - offset
		if toSkip > 0 {
			if _, err := io.CopyN(io.Discard, zr, int64(toSkip)); err != nil {
				return err
			}
		}
		if _, err := io.ReadFull(zr, buf[:s]); err != nil {
			panic(err)
			return err
		}
		offset = o + s
		readLabels(buf[:s], mv)
		dest[h] = mv
	}
	return nil
}

func readV4(reader io.Reader, h *header, from timeseries.Time, pointsCount int, step timeseries.Duration, metrics []string, dest map[uint64]*model.MetricValues, fillFunc timeseries.FillFunc) error {
	var (
		missingLabels []int
		err           error
	)
	gh := &groupChunkHeader{}
	if err = binary.Read(reader, binary.LittleEndian, gh); err != nil {
		return err
	}

	chunkMetrics, err := readMetricNames(reader, gh)
	if err != nil {
		return err
	}
	hashesBuf := make([]byte, int(h.DataSizeOrMetricsCount)*8)
	if _, err = io.ReadFull(reader, hashesBuf); err != nil {
		return err
	}
	hashes := asUint64(hashesBuf)
	br, err := newBlockReader(reader, chunkMetrics, metrics, h)
	if err != nil {
		return err
	}
	defer br.reclaimBuffers()

	for i, hash := range hashes {
		mv, exists := dest[hash]
		if mv == nil {
			mv = &model.MetricValues{
				LabelsHash: hash,
			}
			for range metrics {
				mv.Values = append(mv.Values, timeseries.New(from, pointsCount, step))
			}
			missingLabels = append(missingLabels, i)
		}
		changed, err := br.read(mv, fillFunc)
		if err != nil {
			return err
		}
		if changed && !exists {
			dest[hash] = mv
		}
	}
	if err = readGroupLabels(reader, hashes, missingLabels, dest); err != nil {
		return err
	}
	return nil
}
