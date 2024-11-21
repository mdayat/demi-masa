package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/go-redis/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/twilio/twilio-go"
)

var (
	firebaseApp  *firebase.App
	firebaseAuth *auth.Client
	queries      *repository.Queries
	redisClient  *redis.Client
	twilioClient *twilio.RestClient
	db           *pgxpool.Pool
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Stack().Err(err).Msgf("failed to load %s file", ".env")
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	log.Logger = log.With().Caller().Logger()

	ctx := context.Background()
	db, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to establish a connection to a PostgreSQL server with a connection string")
	}
	defer db.Close()
	queries = repository.New(db)

	firebaseApp, err = firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to initialize firebase app")
	}

	firebaseAuth, err = firebaseApp.Auth(ctx)
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("failed to initialize firebase auth")
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"),
		Password: "",
		DB:       0,
	})

	ACCOUNT_SID := os.Getenv("TWILIO_ACCOUNT_SID")
	AUTH_TOKEN := os.Getenv("TWILIO_AUTH_TOKEN")

	twilioClient = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: ACCOUNT_SID,
		Password: AUTH_TOKEN,
	})

	router := chi.NewRouter()
	router.Use(middleware.CleanPath)
	router.Use(middleware.RealIP)
	router.Use(logger)
	router.Use(middleware.Recoverer)
	router.Use(httprate.LimitByIP(100, 1*time.Minute))
	options := cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTION"},
		AllowedHeaders:   []string{"User-Agent", "Content-Type", "Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Connection", "Host", "Origin", "Referer"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	router.Use(cors.Handler(options))
	router.Use(middleware.AllowContentType("application/json"))
	router.Use(middleware.Heartbeat("/ping"))

	router.Post("/login", loginHandler)
	router.Post("/order/callback", webhookHandler)

	router.Group(func(r chi.Router) {
		r.Use(authenticate)

		r.Get("/users/{userID}", getUserHandler)

		r.Post("/otp/generation", generateOTPHandler)
		r.Post("/otp/verification", verifyOTPHandler)

		r.Post("/order", createOrderHandler)
	})

	http.ListenAndServe(":80", router)
}
