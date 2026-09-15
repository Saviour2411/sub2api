// Package lifecycle 管理发布期间的进程工作、就绪状态和安全退役。
package lifecycle

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type State string

const (
	Starting State = "starting"
	Standby  State = "standby"
	Active   State = "active"
	Draining State = "draining"
	Drained  State = "drained"
)

const (
	HTTP         = "http"
	SSE          = "sse"
	WS           = "ws_connections"
	WSTurn       = "ws_turns"
	Upstream     = "upstream_drains"
	Usage        = "usage_running"
	UsagePending = "usage_pending"
	Producer     = "usage_producers"
	Background   = "background"
)

var ErrBusy = errors.New("实例尚未排空")
var ErrRetired = errors.New("实例已封闭接入")

// Process 是当前进程唯一的生命周期；测试可单独构造 Manager。
var Process = New("i" + strings.ReplaceAll(uuid.NewString(), "-", ""))

type Snapshot struct {
	ID            string           `json:"instance_id"`
	Version       string           `json:"version"`
	State         State            `json:"state"`
	Ready         bool             `json:"ready"`
	Sealed        bool             `json:"sealed"`
	Work          map[string]int64 `json:"work"`
	SessionLeases int              `json:"session_leases"`
}

type Manager struct {
	mu          sync.Mutex
	id, version string
	state       State
	sealed      bool
	work        map[string]int64
	leases      map[string]time.Time
	changed     chan struct{}
	retired     chan struct{}
	probe       func(context.Context) error
	sharedDB    *sql.DB
	affinity    *Affinity
}

func New(id string) *Manager {
	return &Manager{id: id, state: Active, work: make(map[string]int64), leases: make(map[string]time.Time), changed: make(chan struct{}, 1), retired: make(chan struct{})}
}
func (m *Manager) ID() string               { return m.id }
func (m *Manager) Retired() <-chan struct{} { return m.retired }
func (m *Manager) Starting(version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version = version
	m.state = Starting
	m.notify()
}
func (m *Manager) Initialized(standby bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = Active
	if standby {
		m.state = Standby
	}
	m.notify()
}
func (m *Manager) SetProbe(probe func(context.Context) error) {
	m.mu.Lock()
	m.probe = probe
	m.mu.Unlock()
}
func (m *Manager) Check(ctx context.Context) error {
	m.mu.Lock()
	probe := m.probe
	m.mu.Unlock()
	if probe != nil {
		return probe(ctx)
	}
	return nil
}
func (m *Manager) Activate(ctx context.Context) error {
	if err := m.Check(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sealed || (m.state != Standby && m.state != Active && m.state != Draining) {
		return errors.New("实例状态不允许激活")
	}
	m.state = Active
	m.notify()
	return nil
}
func (m *Manager) AcceptingBackground() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == Active && !m.sealed
}

// Begin 仅在退役原子封闭后拒绝；draining 仍允许旧连接和会话续接。
func (m *Manager) Begin(kind string) (func(), error) {
	m.mu.Lock()
	if m.sealed {
		m.mu.Unlock()
		return nil, ErrRetired
	}
	m.work[kind]++
	m.notify()
	m.mu.Unlock()
	return sync.OnceFunc(func() { m.mu.Lock(); defer m.mu.Unlock(); m.work[kind]--; m.notify() }), nil
}
func Track(kind string) func() {
	done, err := Process.Begin(kind)
	if err != nil {
		return func() {}
	}
	return done
}
func (m *Manager) HoldSession(key string, until time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if until.After(m.leases[key]) {
		m.leases[key] = until
		m.notify()
	}
}

// Drain 只暂停新的共享任务，不销毁调度器；退役封闭前仍可安全回滚。
// 已领取任务、残余请求和会话续接继续自然完成。
func (m *Manager) Drain() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sealed || m.state == Draining {
		return
	}
	m.state = Draining
	m.notify()
}
func (m *Manager) idle() bool {
	for key, expiry := range m.leases {
		if !time.Now().Before(expiry) {
			delete(m.leases, key)
		}
	}
	if len(m.leases) > 0 {
		return false
	}
	for _, n := range m.work {
		if n != 0 {
			return false
		}
	}
	return true
}
func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.state
	if state == Draining && m.idle() {
		state = Drained
	}
	work := make(map[string]int64, len(m.work))
	for k, v := range m.work {
		work[k] = v
	}
	return Snapshot{ID: m.id, Version: m.version, State: state, Ready: state == Active && !m.sealed, Sealed: m.sealed, Work: work, SessionLeases: len(m.leases)}
}

// Seal 与 Begin 共用锁，避免看到零计数后新请求入场的退役竞态。
func (m *Manager) Seal() error { return m.seal(nil) }

func (m *Manager) seal(receipt func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sealed {
		return nil
	}
	if m.state != Draining || !m.idle() {
		return ErrBusy
	}
	if receipt != nil {
		if err := receipt(); err != nil {
			return err
		}
	}
	m.sealed = true
	close(m.retired)
	m.notify()
	return nil
}
func (m *Manager) Wait(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		m.mu.Lock()
		idle := m.state == Draining && m.idle()
		ch := m.changed
		m.mu.Unlock()
		if idle {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		case <-ticker.C:
		}
	}
}

// 合并通知而不是为每个请求新建 channel；Wait 的定时检查也会唤醒其他等待者。
func (m *Manager) notify() {
	select {
	case m.changed <- struct{}{}:
	default:
	}
}

func (m *Manager) SetAffinity(a *Affinity) { m.mu.Lock(); m.affinity = a; m.mu.Unlock() }
func (m *Manager) Affinity() *Affinity     { m.mu.Lock(); defer m.mu.Unlock(); return m.affinity }

func (m *Manager) SetSharedDB(db *sql.DB) { m.mu.Lock(); m.sharedDB = db; m.mu.Unlock() }
func (m *Manager) SharedDB() *sql.DB      { m.mu.Lock(); defer m.mu.Unlock(); return m.sharedDB }

// BeginBackground 将状态检查与生产者计数放在同一临界区，封住排空竞态。
func (m *Manager) BeginBackground() (func(), bool) {
	m.mu.Lock()
	if m.state != Active || m.sealed {
		m.mu.Unlock()
		return nil, false
	}
	m.work[Background]++
	m.notify()
	m.mu.Unlock()
	return sync.OnceFunc(func() { m.mu.Lock(); m.work[Background]--; m.notify(); m.mu.Unlock() }), true
}
