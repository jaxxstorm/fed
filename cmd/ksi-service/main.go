package main

import (
	"context"
	"errors"
	"net"
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
	"github.com/jaxxstorm/fed/internal/logging"
	mcpserver "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/providers"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/web"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() (runErr error) {
	mode := os.Getenv("KSI_MODE")
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate", "import-catalog", "provision-runtime", "import-connections", "rotate-connection-keys", "recover-admin":
			mode = "migrate"
		}
	}
	switch mode {
	case "server", "worker", "all", "migrate":
	default:
		mode = "server"
	}
	logger, err := logging.FromEnv(os.Getenv, mode)
	if err != nil {
		logger, _ = logging.New(os.Stdout, os.Stderr, "info", "json", mode)
		logging.Emit(logging.WithLogger(context.Background(), logger), zerolog.ErrorLevel, logging.Fields{Event: "runtime.configuration.failed", Component: "runtime", ErrorCode: "invalid_logging_configuration"})
		return err
	}
	root := logging.WithLogger(context.Background(), logger)
	stage := "configuration"
	logging.Emit(root, zerolog.InfoLevel, logging.Fields{Event: "runtime.starting", Component: "runtime"})
	defer func() {
		level := zerolog.InfoLevel
		outcome := "success"
		if runErr != nil {
			level = zerolog.ErrorLevel
			outcome = "failed"
		}
		logging.Emit(root, level, logging.Fields{Event: "runtime.stopped", Component: "runtime", Stage: stage, Outcome: outcome, ErrorCode: logging.Code(runErr)})
	}()
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	cfg, err := config.FromEnv(func(key string) string {
		if key == "KSI_MODE" && (command == "import-catalog" || command == "migrate" || command == "provision-runtime" || command == "import-connections" || command == "rotate-connection-keys" || command == "recover-admin") {
			return "migrate"
		}
		return os.Getenv(key)
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(root, os.Interrupt, syscall.SIGTERM)
	defer stop()
	stage = "database"
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
		stage = "import_catalog"
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
		_, err = catalog.Import(ctx, db, payload, schema, os.Args[4])
		if err != nil {
			return err
		}
		logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "runtime.catalog.imported", Component: "runtime", Outcome: "success"})
		return nil
	}
	if command == "import-connections" || command == "rotate-connection-keys" {
		stage = command
		return maintainConnections(ctx, db, cfg, command)
	}
	if command == "provision-runtime" {
		stage = "provision_runtime"
		return provisionRuntime(ctx, db, os.Getenv("KSI_RUNTIME_DATABASE_PASSWORD"))
	}
	if command == "recover-admin" {
		stage = "administrator_recovery"
		if len(os.Args) < 3 || len(os.Args) > 4 || (len(os.Args) == 4 && os.Args[3] != "--clear-domains") {
			return errors.New("usage: recover-admin ID [--clear-domains]")
		}
		return auth.RecoverAdministrator(ctx, db, os.Args[2], len(os.Args) == 4)
	}
	if cfg.Mode == "migrate" {
		stage = "migration"
		if err := storage.Migrate(ctx, db); err != nil {
			return errors.New("database migration failed")
		}
		return nil
	}
	var artifacts storage.Artifacts = storage.Files{Directory: cfg.ArtifactDirectory}
	stage = "providers"
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
	connectionService.SetupReadiness = providers.SetupReadiness(os.Getenv)
	for provider, readiness := range connectionService.SetupReadiness {
		outcome, level := "configured", zerolog.InfoLevel
		if !readiness.Ready {
			outcome = "configuration_required"
			level = zerolog.WarnLevel
		}
		logging.Emit(ctx, level, logging.Fields{Event: "runtime.provider.configuration", Component: "runtime", Provider: string(provider), Outcome: outcome, Count: len(readiness.MissingConfiguration)})
	}
	go connectionService.MaintainSetups(ctx)
	// Workers must respect existing bindings even when the setup UI is disabled.
	runner.Connections = connectionService
	if cfg.Mode == "worker" {
		stage = "worker"
		return runner.Run(ctx)
	}
	workerResult := make(chan error, 1)
	if cfg.Mode == "all" {
		go func() { workerResult <- runner.Run(ctx) }()
	}
	var browser *auth.Browser
	stage = "authentication"
	if cfg.DevIdentity {
		actor, err := auth.BootstrapAdmin(ctx, db, "local-development", "developer")
		if err != nil {
			return errors.New("development identity initialization failed; run migrations first")
		}
		browser = &auth.Browser{DB: db, Dev: &actor}
	} else {
		admission := auth.AdmissionConfig{AdminEmail: cfg.AdminEmail, AdminSubject: cfg.AdminSubject, OrganizationClaim: cfg.OIDCOrgClaim, OrganizationValue: cfg.OIDCOrgValue}
		if err := auth.CheckBootstrap(ctx, db, admission); err != nil {
			return errors.New("initial administrator configuration unavailable; configure bootstrap and run migrations")
		}
		if cfg.AdminSubject != "" {
			if _, err := auth.BootstrapAdmin(ctx, db, cfg.OIDCIssuer, cfg.AdminSubject); err != nil {
				return errors.New("administrator provisioning failed")
			}
		}
		browser, err = auth.NewBrowser(ctx, db, cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.PublicURL)
		if err != nil {
			return errors.New("OIDC initialization failed")
		}
		browser.Admission = admission
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
	e.GET("/approval", protected)
	e.GET("/organization", protected)
	e.Any("/services", protected)
	e.Any("/services/*", protected)
	e.Any("/api/ui/v1/*", protected)
	e.GET("/assets/*", protected)
	if cfg.ConnectionsEnabled {
		e.Any("/connections/*", protected)
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: e, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return root }}
	result := make(chan error, 1)
	stage = "listener"
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return errors.New("HTTP listener failed")
	}
	go func() { result <- srv.Serve(listener) }()
	logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "runtime.server.ready", Component: "runtime"})
	select {
	case err := <-workerResult:
		stage = "worker"
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
		logging.Emit(root, zerolog.InfoLevel, logging.Fields{Event: "runtime.shutdown.started", Component: "runtime"})
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
