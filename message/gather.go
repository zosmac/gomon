// Copyright © 2021 The Gomon Project.

package message

import (
	"context"
	"sync"
	"time"
)

// Request returns []Content.
type Request func() []Content

// Gather metrics from each Request, waiting for results for at most timeout duration. If timeout is 0, wait until all requests complete.
func Gather(fs []Request, timeout time.Duration) []Content {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout == 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(fs))
	rs := make(chan []Content, len(fs))

	for _, f := range fs {
		f := f
		go func() {
			defer wg.Done()
			select {
			case rs <- f():
			case <-ctx.Done():
			}
		}()
	}

	go func() {
		wg.Wait()
		close(rs)
	}()

	// Gather available metrics responses

	var ms []Content
	for r := range rs {
		if rs != nil {
			ms = append(ms, r...)
		}
	}

	return ms
}
