package collector

import (
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"k8s.io/klog"
)

const batchFlushConcurrency = 3

type batchColumns interface {
	rows() int
	input() chproto.Input
}

type batch[C batchColumns] struct {
	limit      int
	table      string
	newColumns func() C
	exec       func(query ch.Query) error

	lock    sync.Mutex
	columns C
	spare   chan C
	flushes chan struct{}
	wg      sync.WaitGroup
	done    chan struct{}
}

func (b *batch[C]) start(limit int, timeout time.Duration, table string, newColumns func() C, exec func(query ch.Query) error) {
	b.limit = limit
	b.table = table
	b.newColumns = newColumns
	b.exec = exec
	b.columns = newColumns()
	b.spare = make(chan C, batchFlushConcurrency)
	b.flushes = make(chan struct{}, batchFlushConcurrency)
	b.done = make(chan struct{})

	go func() {
		ticker := time.NewTicker(timeout)
		defer ticker.Stop()
		for {
			select {
			case <-b.done:
				return
			case <-ticker.C:
				b.lock.Lock()
				b.save()
				b.lock.Unlock()
				if len(b.spare) > 1 {
					select {
					case <-b.spare:
					default:
					}
				}
			}
		}
	}()
}

func (b *batch[C]) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
	b.wg.Wait()
}

func (b *batch[C]) write(appendRows func(columns C)) {
	b.lock.Lock()
	defer b.lock.Unlock()
	appendRows(b.columns)
	if b.columns.rows() < b.limit {
		return
	}
	b.save()
}

func (b *batch[C]) save() {
	if b.columns.rows() == 0 {
		return
	}
	columns := b.columns
	b.flushes <- struct{}{}
	select {
	case b.columns = <-b.spare:
	default:
		b.columns = b.newColumns()
	}
	b.wg.Go(func() {
		input := columns.input()
		err := b.exec(ch.Query{Body: input.Into(b.table), Input: input})
		if err != nil {
			klog.Errorln(err)
		}
		for _, i := range input {
			i.Data.(chproto.Resettable).Reset()
		}
		select {
		case b.spare <- columns:
		default:
		}
		<-b.flushes
	})
}
