package main

//go:generate go run github.com/google/wire/cmd/wire

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/setup"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
)

//go:embed VERSION
var embeddedVersion string

// Build-time variables (can be set by ldflags)
var (
	Version   = ""
	Commit    = "unknown"
	Date      = "unknown"
	BuildType = "source" // "source" for manual builds, "release" for CI builds (set by ldflags)
)

func init() {
	// 如果 Version 已通过 ldflags 注入（例如 -X main.Version=...），则不要覆盖。
	if strings.TrimSpace(Version) != "" {
		return
	}

	// 默认从 embedded VERSION 文件读取版本号（编译期打包进二进制）。
	Version = strings.TrimSpace(embeddedVersion)
	if Version == "" {
		Version = "0.0.0-dev"
	}
}

// initLogger configures the default slog handler based on gin.Mode().
// In non-release mode, Debug level logs are enabled.
func main() {
	logger.InitBootstrap()
	defer logger.Sync()

	// Parse command line flags
	setupMode := flag.Bool("setup", false, "Run setup wizard in CLI mode")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		log.Printf("Sub2API %s (commit: %s, built: %s)\n", Version, Commit, Date)
		return
	}

	// CLI setup mode
	if *setupMode {
		if err := setup.RunCLI(); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		return
	}

	// Check if setup is needed
	if setup.NeedsSetup() {
		// Check if auto-setup is enabled (for Docker deployment)
		if setup.AutoSetupEnabled() {
			log.Println("Auto setup mode enabled...")
			if err := setup.AutoSetupFromEnv(); err != nil {
				log.Fatalf("Auto setup failed: %v", err)
			}
			// Continue to main server after auto-setup
		} else {
			log.Println("First run detected, starting setup wizard...")
			runSetupServer()
			return
		}
	}

	// Normal server mode
	runMainServer()
}

func runSetupServer() {
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(config.CORSConfig{}))
	r.Use(middleware.SecurityHeaders(config.CSPConfig{Enabled: true, Policy: config.DefaultCSPPolicy}, nil))

	// Register setup routes
	setup.RegisterRoutes(r)

	// Serve embedded frontend if available
	if web.HasEmbeddedFrontend() {
		r.Use(web.ServeEmbeddedFrontend())
	}

	// Get server address from config.yaml or environment variables (SERVER_HOST, SERVER_PORT)
	// This allows users to run setup on a different address if needed
	addr := config.GetServerAddress()
	log.Printf("Setup wizard available at http://%s", addr)
	log.Println("Complete the setup wizard to configure Sub2API")

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
		Protocols:         protocols,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Failed to start setup server: %v", err)
	}
}

func runMainServer() {
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := logger.Init(logger.OptionsFromConfig(cfg.Log)); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	if cfg.RunMode == config.RunModeSimple {
		log.Println("⚠️  WARNING: Running in SIMPLE mode - billing and quota checks are DISABLED")
	}

	buildInfo := handler.BuildInfo{
		Version:   Version,
		BuildType: BuildType,
	}

	lifecycle.Process.SetLegacyCoexistence(cfg.Lifecycle.Legacy)
	lifecycle.Process.Starting(Version)
	app, err := initializeApplication(buildInfo)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	cfg = app.Config
	defer app.Cleanup()
	if app.PluginManager != nil {
		if err := app.PluginManager.Start(context.Background()); err != nil {
			log.Printf("Plugin manager started in degraded state: %v", err)
		}
	}
	if app.PromptAudit != nil {
		if err := app.PromptAudit.Start(context.Background()); err != nil {
			// Startup continues so unrelated APIs stay up. Fail-closed (unavailable)
			// applies only when a persisted blocking policy was observed; without
			// blocking intent, Prompt Audit stays ModeOff so the gateway remains
			// usable and administrators can still disable the feature (#4560).
			log.Printf("Prompt Audit started in degraded state: %v", err)
		}
	}

	registry := lifecycle.Registry{Redis: app.Redis}
	var leaseHealthy atomic.Bool
	leaseHealthy.Store(true)
	lifecycle.Process.SetProbe(func(ctx context.Context) error {
		if !leaseHealthy.Load() {
			return fmt.Errorf("instance lease unavailable")
		}
		if err := app.DB.PingContext(ctx); err != nil {
			return err
		}
		return app.Redis.Ping(ctx).Err()
	})
	if cfg.Lifecycle.Socket != "" {
		lifecycle.Process.SetAffinity(&lifecycle.Affinity{Registry: registry, Manager: lifecycle.Process, SocketDir: filepath.Dir(cfg.Lifecycle.Socket), Legacy: cfg.Lifecycle.Legacy, Secret: []byte(cfg.JWT.Secret)})
	}
	closeControl, err := lifecycle.Process.Control(cfg.Lifecycle.Socket, app.Server.Handler)
	if err != nil {
		log.Fatalf("Failed to initialize lifecycle control: %v", err)
	}
	defer closeControl()
	if cfg.Lifecycle.Socket != "" {
		instance := lifecycle.Instance{ID: lifecycle.Process.ID(), Socket: cfg.Lifecycle.Socket, Version: Version}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = registry.Publish(ctx, instance)
		cancel()
		if err != nil {
			log.Fatalf("Failed to register instance: %v", err)
		}
		heartbeatCtx, stopHeartbeat := context.WithCancel(context.Background())
		// 心跳只能在请求和清理全部完成后停止。
		defer stopHeartbeat()
		go registry.Heartbeat(heartbeatCtx, instance, func(err error) { leaseHealthy.Store(err == nil) })
	}
	listener, err := net.Listen("tcp", app.Server.Addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	lifecycle.Process.Initialized(cfg.Lifecycle.Mode == "standby")
	go func() {
		if err := app.Server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP serve failed: %v", err)
		}
	}()
	log.Printf("Server started on %s instance=%s", app.Server.Addr, lifecycle.Process.ID())
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	select {
	case <-quit:
	case <-lifecycle.Process.Retired():
	}
	log.Println("Draining server without cancelling active work...")
	lifecycle.Process.Drain()
	// Shutdown不等待hijacked连接；生命周期额外等待WS、用量和脱离客户端的工作。
	httpDone := make(chan struct{})
	go func() { defer close(httpDone); _ = app.Server.Shutdown(context.Background()) }()
	_ = lifecycle.Process.Wait(context.Background())
	<-httpDone
	app.Cleanup()
	log.Println("Server exited")
}
