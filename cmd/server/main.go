package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	envconfig "github.com/caarlos0/env/v6"
	"github.com/jackc/pgx/v4"
	_ "github.com/joho/godotenv/autoload"
	"github.com/ory/graceful"
	"github.com/ramin0/google-calendar-push/internal/handler"
	"github.com/ramin0/google-calendar-push/internal/store"
	"github.com/ramin0/google-calendar-push/migrations"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
)

var env struct {
	SERVER_PORT        int    `env:"PORT" envDefault:"3000"`
	LOG_ENVIRONMENT    string `env:"LOG_ENVIRONMENT" envDefault:"production"`
	DATABASE_URL       string `env:"DATABASE_URL,notEmpty,unset"`
	DATABASE_LOG_LEVEL string `env:"DATABASE_LOG_LEVEL" envDefault:"none"`
	AUTH_CLIENT_ID     string `env:"AUTH_CLIENT_ID,notEmpty"`
	AUTH_CLIENT_SECRET string `env:"AUTH_CLIENT_SECRET,notEmpty,unset"`
}

func main() {
	if err := envconfig.Parse(&env); err != nil {
		panic(err)
	}

	logcfg := zap.NewProductionConfig()
	if strings.EqualFold(env.LOG_ENVIRONMENT, "development") {
		logcfg = zap.NewDevelopmentConfig()
		logcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	log, err := logcfg.Build()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	ctx := context.Background()
	dbcfg, err := pgx.ParseConfig(env.DATABASE_URL)
	if err != nil {
		log.Fatal("failed tp parse database uri", zap.Error(err))
	}
	dbcfg.LogLevel, err = pgx.LogLevelFromString(env.DATABASE_LOG_LEVEL)
	if err != nil {
		log.Fatal("failed to parse database log level", zap.Error(err))
	}
	dbcfg.Logger = &pgxLogger{log.Named("db")}
	db, err := pgx.ConnectConfig(ctx, dbcfg)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close(ctx)

	if err := migrations.Migrate(ctx, db); err != nil {
		log.Fatal("failed to migrate database", zap.Error(err))
	}

	store := store.New(db)

	handler := handler.New(env.AUTH_CLIENT_ID, env.AUTH_CLIENT_SECRET)
	http.HandleFunc("/auth", handler.Auth(log.Named("handler.auth")))
	http.HandleFunc("/auth/callback", handler.AuthCallback(log.Named("handler.auth_callback"), store))
	http.HandleFunc("/webhook", handler.Webhook(log.Named("handler.webhook"), store))

	s := &http.Server{Addr: fmt.Sprintf(":%d", env.SERVER_PORT)}
	log.Info("Listening on " + s.Addr)
	if err := graceful.Graceful(s.ListenAndServe, s.Shutdown); err != nil &&
		!errors.Is(err, context.DeadlineExceeded) {
		log.Fatal("failed to graceful start/shutdown server", zap.Error(err))
	}
}
