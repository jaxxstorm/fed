package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/checks"
	"github.com/jaxxstorm/fed/internal/config"
	"github.com/jaxxstorm/fed/internal/connections"
	mcpserver "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/providers"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/web"
	"github.com/labstack/echo/v4"
)

func main() {
	if err := run(); err != nil {
		// Driver errors can contain connection details: do not log them here.
		slog.Error("service stopped", "reason", err.Error())
		os.Exit(1)
	}
}

func run() error {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	cfg, err := config.FromEnv(func(key string) string {
		if key == "KSI_MODE" && (command == "import-catalog" || command == "migrate" || command == "provision-runtime" || command == "import-connections" || command == "rotate-connection-keys") {
			return "migrate"
		}
		return os.Getenv(key)
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("database configuration failed")
	}
	defer db.Close()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = db.Ping(startup)
	cancel()
	if err != nil {
		return errors.New("database connection failed")
	}
	if command == "import-catalog" {
		if len(os.Args) != 5 {
			return errors.New("usage: ksi-service import-catalog rules.json schema.json commit-sha")
		}
		payload, err := os.ReadFile(os.Args[2])
		if err != nil {
			return errors.New("cannot read rules file")
		}
		schema, err := os.ReadFile(os.Args[3])
		if err != nil {
			return errors.New("cannot read schema file")
		}
		id, err := catalog.Import(ctx, db, payload, schema, os.Args[4])
		if err != nil {
			return err
		}
		slog.Info("catalog imported", "catalog_id", id)
		return nil
	}
	if command == "import-connections" || command == "rotate-connection-keys" {
		return maintainConnections(ctx, db, cfg, command)
	}
	if command == "provision-runtime" {
		return provisionRuntime(ctx, db, os.Getenv("KSI_RUNTIME_DATABASE_PASSWORD"))
	}
	if cfg.Mode == "migrate" {
		if err := storage.Migrate(ctx, db); err != nil {
			return errors.New("database migration failed")
		}
		return nil
	}
	var artifacts storage.Artifacts = storage.Files{Directory: cfg.ArtifactDirectory}
	if cfg.ArtifactBucket != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return errors.New("artifact credentials configuration failed")
		}
		client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if cfg.ArtifactEndpoint != "" {
				o.BaseEndpoint = &cfg.ArtifactEndpoint
				o.UsePathStyle = true
			}
		})
		artifacts = storage.S3{Client: client, Bucket: cfg.ArtifactBucket}
	}
	runner := &checks.Runner{DB: db, Artifacts: artifacts, Registry: checks.Registry(os.Getenv("GITHUB_TOKEN"))}
	adapters, err := providers.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	registry, _ := connections.NewRegistry(adapters)
	keys, _ := connections.LoadKeyring()
	connectionService := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db, Keys: keys}, Artifacts: artifacts}
	// Workers must respect existing bindings even when the setup UI is disabled.
	runner.Connections = connectionService
	if cfg.Mode == "worker" {
		return runner.Run(ctx)
	}
	workerResult := make(chan error, 1)
	if cfg.Mode == "all" {
		go func() { workerResult <- runner.Run(ctx) }()
	}
	var browser *auth.Browser
	if cfg.DevIdentity {
		actor, err := auth.BootstrapAdmin(ctx, db, "local-development", "developer")
		if err != nil {
			return errors.New("development identity initialization failed; run migrations first")
		}
		browser = &auth.Browser{DB: db, Dev: &actor}
	} else {
		if _, err := auth.BootstrapAdmin(ctx, db, cfg.OIDCIssuer, cfg.AdminSubject); err != nil {
			return errors.New("administrator provisioning failed")
		}
		browser, err = auth.NewBrowser(ctx, db, cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.PublicURL)
		if err != nil {
			return errors.New("OIDC initialization failed")
		}
	}
	e := web.Bootstrap(db.Ping)
	store := &assessment.Store{DB: db, Artifacts: artifacts}
	e.GET("/auth/login", func(c echo.Context) error { browser.Login(c.Response(), c.Request()); return nil })
	e.GET("/auth/callback", func(c echo.Context) error { browser.Callback(c.Response(), c.Request()); return nil })
	e.POST("/auth/logout", echo.WrapHandler(browser.Protect(http.HandlerFunc(browser.Logout))))
	app := &web.App{Store: store, Runner: runner}
	if cfg.ConnectionsEnabled {
		app.Connections = connectionService
		app.ConnectionOrigin = cfg.PublicURL
	}
	e.Any("/mcp", echo.WrapHandler(mcpserver.Handler(store, runner, mcpserver.ConnectionOptions{Service: app.Connections, PublicOrigin: cfg.PublicURL})))
	protected := echo.WrapHandler(browser.Protect(app.Handler()))
	e.GET("/", protected)
	e.Any("/services", protected)
	e.Any("/services/*", protected)
	e.Any("/api/ui/v1/*", protected)
	e.GET("/assets/*", protected)
	if cfg.ConnectionsEnabled {
		e.Any("/connections/*", protected)
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: e, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	result := make(chan error, 1)
	go func() { result <- srv.ListenAndServe() }()
	slog.Info("server started", "address", cfg.Listen, "development_identity", cfg.DevIdentity)
	select {
	case err := <-workerResult:
		stop()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
		if err != nil {
			return errors.New("background worker failed")
		}
		return nil
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP listener failed")
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
