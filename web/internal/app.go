package internal

import (
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/mdayat/demi-masa/web/configs/env"
)

func InitApp() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.CleanPath)
	router.Use(middleware.RealIP)
	router.Use(logger)
	router.Use(middleware.Recoverer)
	router.Use(httprate.LimitByIP(100, 1*time.Minute))
	options := cors.Options{
		AllowedOrigins:   strings.Split(env.ALLOWED_ORIGINS, ","),
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"},
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
