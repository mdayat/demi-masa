package httpservice

import (
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/odemimasa/backend/internal/services"
	"github.com/odemimasa/backend/repository"
	"github.com/redis/go-redis/v9"
	"github.com/twilio/twilio-go"
)

var (
	db             *pgxpool.Pool
	queries        *repository.Queries
	firebaseAuth   *auth.Client
	redisClient    *redis.Client
	twilioClient   *twilio.RestClient
	asynqClient    *asynq.Client
	asynqInspector *asynq.Inspector
)

func New() *chi.Mux {
	db = services.GetDB()
	queries = services.GetQueries()
	firebaseAuth = services.GetFirebaseAuth()
	redisClient = services.GetRedis()
	twilioClient = services.GetTwilio()
	asynqClient = services.GetAsynqClient()
	asynqInspector = services.GetAsynqInspector()

	router := chi.NewRouter()
	router.Use(middleware.CleanPath)
	router.Use(middleware.RealIP)
	router.Use(logger)
	router.Use(middleware.Recoverer)
	router.Use(httprate.LimitByIP(100, 1*time.Minute))
	options := cors.Options{
		AllowedOrigins:   []string{"https://www.demi-masa.id"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTION"},
		AllowedHeaders:   []string{"User-Agent", "Content-Type", "Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Connection", "Host", "Origin", "Referer", "Authorization"},
		ExposedHeaders:   []string{"Content-Length", "Location"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	router.Use(cors.Handler(options))
	router.Use(middleware.Heartbeat("/ping"))

	router.Post("/login", loginHandler)
	router.Post("/transactions/callback", tripayWebhookHandler)

	router.Group(func(r chi.Router) {
		r.Use(authenticate)

		r.Delete("/users/{userID}", deleteUserHandler)
		r.Put("/users/{userID}/time-zone", updateTimeZoneHandler)

		r.Post("/otp/generation", generateOTPHandler)
		r.Post("/otp/verification", verifyOTPHandler)

		r.Get("/transactions", getTransactionsHandler)
		r.Post("/transactions", createTxHandler)

		r.Get("/tasks", getTasksHandler)
		r.Post("/tasks", createTaskHandler)
		r.Put("/tasks/{taskID}", updateTaskHandler)
		r.Delete("/tasks/{taskID}", deleteTaskHandler)

		r.Get("/prayers", getPrayersHandler)
		r.Get("/prayers/today", getTodayPrayersHandler)
		r.Put("/prayers/{prayerID}", updatePrayerHandler)

		r.Get("/subscription-plans", getSubsPlansHandler)
	})

	return router
}
