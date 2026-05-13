package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/keploy/taskmanager/internal/auth"
	"github.com/keploy/taskmanager/internal/cache"
	"github.com/keploy/taskmanager/internal/config"
	"github.com/keploy/taskmanager/internal/database"
	"github.com/keploy/taskmanager/internal/external"
	"github.com/keploy/taskmanager/internal/grpcserver"
	"github.com/keploy/taskmanager/internal/handlers"
	"github.com/keploy/taskmanager/internal/logger"
	"github.com/keploy/taskmanager/internal/repository"
	"github.com/keploy/taskmanager/internal/service"
	"github.com/keploy/taskmanager/proto/taskpb"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel)
	log.Info("starting taskmanager",
		"rest_port", cfg.RESTPort, "grpc_port", cfg.GRPCPort,
		"mysql_host", cfg.MySQL.Host, "redis_host", cfg.Redis.Host)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := database.Connect(ctx, cfg.MySQL)
	if err != nil {
		log.Error("mysql connect failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	rcache, err := cache.New(ctx, cfg.Redis)
	if err != nil {
		log.Error("redis connect failed", "err", err)
		os.Exit(1)
	}
	defer rcache.Close()

	extClient := external.NewClient(cfg.ExternalAPI)
	jm := auth.NewManager(cfg.JWTSecret, cfg.JWTExpiry)

	userRepo := repository.NewUserRepo(db)
	taskRepo := repository.NewTaskRepo(db)

	authSvc := service.NewAuthService(userRepo, jm)
	userSvc := service.NewUserService(userRepo, rcache, extClient)
	taskSvc := service.NewTaskService(taskRepo, userRepo, rcache)

	if err := bootstrapAdmin(ctx, authSvc, userRepo, log); err != nil {
		log.Warn("admin bootstrap", "err", err)
	}

	router := (&handlers.Router{
		Auth:  handlers.NewAuthHandler(authSvc),
		Users: handlers.NewUserHandler(userSvc),
		Tasks: handlers.NewTaskHandler(taskSvc),
	}).Build(jm, log)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.RESTPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	gs := grpc.NewServer(grpc.ForceServerCodec(encodingCodec()))
	taskpb.RegisterAuthServiceServer(gs, grpcserver.NewAuthGRPC(authSvc))
	taskpb.RegisterTaskServiceServer(gs, grpcserver.NewTaskGRPC(taskSvc, jm))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		log.Info("REST listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("rest server", "err", err)
			cancel()
		}
	}()

	go func() {
		defer wg.Done()
		l, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			log.Error("grpc listen", "err", err)
			cancel()
			return
		}
		log.Info("gRPC listening", "addr", l.Addr().String())
		if err := gs.Serve(l); err != nil {
			log.Error("grpc server", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutdown signal received")

	shutdownCtx, sc := context.WithTimeout(context.Background(), 10*time.Second)
	defer sc()
	_ = httpSrv.Shutdown(shutdownCtx)
	gs.GracefulStop()

	wg.Wait()
	log.Info("bye")
}

// bootstrapAdmin creates a default admin if none exists, so the demo can run end-to-end.
func bootstrapAdmin(ctx context.Context, a *service.AuthService, ur *repository.UserRepo, log *slog.Logger) error {
	username := os.Getenv("BOOTSTRAP_ADMIN_USERNAME")
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "admin12345"
	}
	if existing, err := ur.GetByUsername(ctx, username); err == nil && existing != nil {
		log.Info("admin exists", "username", username)
		return nil
	}
	ext := 1
	_, err := a.Register(ctx, service.RegisterInput{
		Username: username, Email: username + "@local",
		Password: password, Role: "admin", ExternalID: &ext,
	})
	if err != nil {
		return err
	}
	log.Info("bootstrapped admin", "username", username)
	return nil
}
