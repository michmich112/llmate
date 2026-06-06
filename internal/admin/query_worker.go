package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/llmate/gateway/internal/db"
)

type QueryWorker struct {
	store db.Store
	ch    chan queryJob
}

type queryJob struct {
	fn     func(ctx context.Context, store db.Store) (any, error)
	result chan queryResult
}

type queryResult struct {
	val any
	err error
}

func NewQueryWorker(store db.Store, bufferSize int) *QueryWorker {
	w := &QueryWorker{store: store, ch: make(chan queryJob, bufferSize)}
	return w
}

func (w *QueryWorker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-w.ch:
				val, err := job.fn(ctx, w.store)
				job.result <- queryResult{val: val, err: err}
			}
		}
	}()
}

func (w *QueryWorker) Run(ctx context.Context, fn func(ctx context.Context, store db.Store) (any, error)) (any, error) {
	resCh := make(chan queryResult, 1)
	select {
	case w.ch <- queryJob{fn: fn, result: resCh}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case res := <-resCh:
		return res.val, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("admin query timed out")
	}
}
