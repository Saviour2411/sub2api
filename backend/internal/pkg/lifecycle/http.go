package lifecycle

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (m *Manager) HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(peerHeader) != "" && r.Context().Value(peerContextKey{}) != true {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/readyz" {
			w.Header().Set("X-Sub2api-Instance", m.ID())
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if !m.Snapshot().Ready || m.Check(ctx) != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		state := m.Snapshot().State
		if state == Starting || state == Standby {
			http.Error(w, "not active", http.StatusServiceUnavailable)
			return
		}
		done, err := m.Begin(HTTP)
		if err != nil {
			http.Error(w, "retired", http.StatusServiceUnavailable)
			return
		}
		defer done()
		rw := &responseWriter{ResponseWriter: w, manager: m}
		defer func() {
			if rw.sseDone != nil {
				rw.sseDone()
			}
		}()
		next.ServeHTTP(rw, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	manager *Manager
	sseDone func()
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) observe() {
	if w.sseDone == nil && strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		w.sseDone, _ = w.manager.Begin(SSE)
	}
}
func (w *responseWriter) WriteHeader(status int)      { w.observe(); w.ResponseWriter.WriteHeader(status) }
func (w *responseWriter) Write(b []byte) (int, error) { w.observe(); return w.ResponseWriter.Write(b) }
func (w *responseWriter) Flush() {
	w.observe()
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err != nil {
		return nil, nil, err
	}
	done, err := w.manager.Begin(WS)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return &trackedConn{Conn: conn, done: done}, rw, nil
}

type trackedConn struct {
	net.Conn
	once sync.Once
	done func()
}

func (c *trackedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.done); return err }

// Control 仅监听实例专属 Unix socket；不把管理操作挂到业务端口。
func (m *Manager) Control(socket string, peer ...http.Handler) (func(), error) {
	if socket == "" {
		return func() {}, nil
	}
	if !filepath.IsAbs(socket) {
		return nil, errors.New("生命周期 socket 必须为绝对路径")
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0700); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	} // 不删除已有 socket，避免覆盖仍运行的实例。
	if err = os.Chmod(socket, 0600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	mux := http.NewServeMux()
	if len(peer) > 0 && peer[0] != nil {
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(peerHeader) == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), peerContextKey{}, true))
			affinity := m.Affinity()
			if affinity == nil || affinity.AuthenticateTransport(r) != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			peer[0].ServeHTTP(w, r)
		}))
	}
	mux.HandleFunc("GET /synthetic", func(w http.ResponseWriter, r *http.Request) {
		if err := m.Check(r.Context()); err != nil {
			http.Error(w, "dependencies unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []string{"event: probe\ndata: ready\n\n", "event: complete\ndata: ok\n\n"} {
			_, _ = w.Write([]byte(event))
			_ = http.NewResponseController(w).Flush()
		}
	})
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		s := m.Snapshot()
		if m.Check(ctx) != nil {
			s.Ready = false
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("GET /check", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := m.Check(ctx); err != nil {
			http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /activate", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := m.Activate(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /drain", func(w http.ResponseWriter, _ *http.Request) { m.Drain(); w.WriteHeader(202) })
	mux.HandleFunc("POST /retire", func(w http.ResponseWriter, _ *http.Request) {
		if err := m.seal(func() error { return m.writeRetiredReceipt(socket) }); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(204)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return func() { _ = server.Close(); _ = os.Remove(socket) }, nil
}

// 退役确认先落盘再通知主程序退出，用于处理控制响应丢失/部署脚本中断后的安全恢复。
func (m *Manager) writeRetiredReceipt(socket string) error {
	path := socket + ".retired"
	file, err := os.OpenFile(path+".next", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := json.NewEncoder(file).Encode(map[string]any{"instance_id": m.ID(), "sealed": true}); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(path+".next", path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
