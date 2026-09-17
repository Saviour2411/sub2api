package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

type cleanupContextBody struct {
	ctx              context.Context
	reader           io.Reader
	waitAfterPayload bool
	closeCalls       atomic.Int32
	closeErr         error
}

func (body *cleanupContextBody) Read(buffer []byte) (int, error) {
	if body.reader != nil {
		count, err := body.reader.Read(buffer)
		if err != io.EOF || !body.waitAfterPayload {
			return count, err
		}
	}
	<-body.ctx.Done()
	return 0, body.ctx.Err()
}

func (body *cleanupContextBody) Close() error {
	body.closeCalls.Add(1)
	<-body.ctx.Done()
	return body.closeErr
}

func TestFirstTokenCleanupCancelsBeforeConcurrentClose(tester *testing.T) {
	synctest.Test(tester, func(tester *testing.T) {
		ctx, cancel := context.WithCancel(tester.Context())
		defer cancel()
		closeErr := errors.New("上游关闭错误")
		upstream := &cleanupContextBody{ctx: ctx, closeErr: closeErr}
		var cleanupCalls atomic.Int32
		body := &firstTokenCleanupReadCloser{upstream: upstream, cleanup: func() {
			cleanupCalls.Add(1)
			cancel()
		}}
		readDone := make(chan error, 1)
		go func() {
			_, err := body.Read(make([]byte, 1))
			readDone <- err
		}()
		synctest.Wait()
		closed := make(chan error, 8)
		for range cap(closed) {
			go func() { closed <- body.Close() }()
		}
		synctest.Wait()
		cancelledByClose := ctx.Err()
		cancel()
		synctest.Wait()
		require.ErrorIs(tester, cancelledByClose, context.Canceled, "关闭响应体前必须取消本轮上游请求")
		require.ErrorIs(tester, <-readDone, context.Canceled)
		for range cap(closed) {
			require.ErrorIs(tester, <-closed, closeErr)
		}
		require.EqualValues(tester, 1, upstream.closeCalls.Load())
		require.EqualValues(tester, 1, cleanupCalls.Load())
	})
}

func TestFirstTokenCleanupPreservesBodyUntilReadCompletes(tester *testing.T) {
	for _, readErr := range []error{io.EOF, io.ErrUnexpectedEOF} {
		tester.Run(readErr.Error(), func(tester *testing.T) {
			ctx, cancel := context.WithCancel(tester.Context())
			defer cancel()
			payload := `{"error":{"message":"上游错误详情"}}`
			upstream := &cleanupContextBody{ctx: ctx, reader: io.MultiReader(strings.NewReader(payload), &cleanupErrorReader{err: readErr})}
			var cleanupCalls atomic.Int32
			body := &firstTokenCleanupReadCloser{upstream: upstream, cleanup: func() {
				cleanupCalls.Add(1)
				cancel()
			}}
			buffer := make([]byte, len(payload))
			_, err := io.ReadFull(body, buffer)
			require.NoError(tester, err)
			require.Equal(tester, payload, string(buffer))
			require.NoError(tester, ctx.Err(), "HTTP错误正文读完之前不能取消请求")
			_, err = body.Read(buffer)
			require.ErrorIs(tester, err, readErr)
			require.ErrorIs(tester, ctx.Err(), context.Canceled)
			require.NoError(tester, body.Close())
			require.NoError(tester, body.Close())
			require.EqualValues(tester, 1, upstream.closeCalls.Load())
			require.EqualValues(tester, 1, cleanupCalls.Load())
		})
	}
}

type cleanupErrorReader struct{ err error }

func (reader *cleanupErrorReader) Read([]byte) (int, error) { return 0, reader.err }
