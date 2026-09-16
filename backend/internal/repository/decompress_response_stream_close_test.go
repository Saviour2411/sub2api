package repository

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestDecompressResponseBodyStreamCloseDoesNotWaitForEOF(tester *testing.T) {
	payload := []byte("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	for _, protocol := range []string{"HTTP/1.1", "HTTP/2.0"} {
		for _, encoding := range []string{"identity", "gzip", "br", "deflate", "zstd"} {
			for _, scenario := range []struct {
				name           string
				concurrentRead bool
			}{
				{name: "完整终止"},
				{name: "读取中关闭", concurrentRead: true},
			} {
				tester.Run(protocol+"/"+encoding+"/"+scenario.name, func(tester *testing.T) {
					encoded := flushedStreamPayload(tester, encoding, payload)
					release := make(chan struct{})
					server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
						writer.Header().Set("Content-Type", "text/event-stream")
						writer.Header().Set("Content-Encoding", encoding)
						_, _ = writer.Write(encoded)
						_ = http.NewResponseController(writer).Flush()
						select {
						case <-request.Context().Done():
						case <-release:
						}
					}))
					server.EnableHTTP2 = protocol == "HTTP/2.0"
					server.StartTLS()
					tester.Cleanup(func() {
						close(release)
						server.Close()
					})
					client := server.Client()
					client.Timeout = 5 * time.Second
					request, err := http.NewRequestWithContext(tester.Context(), http.MethodGet, server.URL, nil)
					require.NoError(tester, err)
					request.Header.Set("Accept-Encoding", encoding)
					response, err := client.Do(request)
					require.NoError(tester, err)
					require.Equal(tester, protocol, response.Proto)
					originalBody := response.Body
					tester.Cleanup(func() { _ = originalBody.Close() })
					decompressResponseBody(response)
					actual := make([]byte, len(payload))
					_, err = io.ReadFull(response.Body, actual)
					require.NoError(tester, err)
					require.Equal(tester, payload, actual, "最终用量和终止事件必须完整保留")

					readDone := make(chan error, 1)
					if scenario.concurrentRead {
						readStarted := make(chan struct{})
						go func() {
							close(readStarted)
							_, readErr := response.Body.Read(make([]byte, 1))
							readDone <- readErr
						}()
						<-readStarted
					}
					closed := make(chan error, 1)
					go func() { closed <- response.Body.Close() }()
					select {
					case err := <-closed:
						require.NoError(tester, err)
					case <-time.After(time.Second):
						_ = originalBody.Close()
						select {
						case <-closed:
						case <-time.After(time.Second):
							tester.Fatal("关闭原始网络响应体后，解压器仍未退出")
						}
						tester.Fatal("完整流终止事件后，关闭响应体不应等待上游EOF")
					}
					if scenario.concurrentRead {
						select {
						case err := <-readDone:
							require.Error(tester, err)
						case <-time.After(time.Second):
							tester.Fatal("关闭响应体后读取协程必须退出")
						}
					}
				})
			}
		}
	}
}

func flushedStreamPayload(tester *testing.T, encoding string, payload []byte) []byte {
	tester.Helper()
	var buffer bytes.Buffer
	var encoder interface {
		io.WriteCloser
		Flush() error
	}
	switch encoding {
	case "gzip":
		encoder = gzip.NewWriter(&buffer)
	case "br":
		encoder = brotli.NewWriter(&buffer)
	case "deflate":
		var err error
		encoder, err = flate.NewWriter(&buffer, flate.DefaultCompression)
		require.NoError(tester, err)
	case "zstd":
		var err error
		encoder, err = zstd.NewWriter(&buffer)
		require.NoError(tester, err)
	default:
		return payload
	}
	_, err := encoder.Write(payload)
	require.NoError(tester, err)
	require.NoError(tester, encoder.Flush())
	encoded := bytes.Clone(buffer.Bytes())
	require.NoError(tester, encoder.Close())
	return encoded
}

type streamCloseDecoder struct {
	readStarted chan struct{}
	readRelease chan struct{}
	readDone    chan struct{}
	closeCalls  atomic.Int32
}

func (decoder *streamCloseDecoder) Read([]byte) (int, error) {
	close(decoder.readStarted)
	<-decoder.readRelease
	close(decoder.readDone)
	return 0, io.ErrClosedPipe
}

func (decoder *streamCloseDecoder) Close() error {
	<-decoder.readDone
	decoder.closeCalls.Add(1)
	return errors.New("解压器关闭错误")
}

type streamCloseNetworkBody struct {
	release    chan struct{}
	once       sync.Once
	closeCalls atomic.Int32
	err        error
}

func (body *streamCloseNetworkBody) Close() error {
	body.closeCalls.Add(1)
	body.once.Do(func() { close(body.release) })
	return body.err
}

func TestDecompressedBodyCloseUnblocksReadOnce(tester *testing.T) {
	for _, scenario := range []struct {
		name     string
		closeErr error
	}{
		{name: "并发关闭只释放一次"},
		{name: "保留网络关闭错误并释放解压器", closeErr: errors.New("网络响应体关闭错误")},
	} {
		tester.Run(scenario.name, func(tester *testing.T) {
			decoder := &streamCloseDecoder{readStarted: make(chan struct{}), readRelease: make(chan struct{}), readDone: make(chan struct{})}
			network := &streamCloseNetworkBody{release: decoder.readRelease, err: scenario.closeErr}
			tester.Cleanup(func() { _ = network.Close() })
			body := &decompressedBody{reader: decoder, closer: network}
			readDone := make(chan error, 1)
			go func() {
				_, err := body.Read(make([]byte, 1))
				readDone <- err
			}()
			<-decoder.readStarted
			closed := make(chan error, 2)
			for range 2 {
				go func() { closed <- body.Close() }()
			}
			for range 2 {
				select {
				case err := <-closed:
					require.ErrorIs(tester, err, scenario.closeErr)
				case <-time.After(time.Second):
					tester.Fatal("关闭网络响应体必须先解除正在进行的读取")
				}
			}
			require.ErrorIs(tester, <-readDone, io.ErrClosedPipe)
			require.EqualValues(tester, 1, network.closeCalls.Load())
			require.EqualValues(tester, 1, decoder.closeCalls.Load())
		})
	}
}
