package internal

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	"github.com/mdayat/demi-masa/asynqmon/configs/env"
)

func InitApp() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.CleanPath)
	router.Use(middleware.RealIP)
	router.Use(logger)
	router.Use(middleware.Recoverer)
	router.Use(httprate.LimitByIP(100, 1*time.Minute))
	options := cors.Options{
		AllowedOrigins:   []string{env.ASYNQMON_BASE_URL},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTION"},
		AllowedHeaders:   []string{"User-Agent", "Content-Type", "Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Connection", "Host", "Origin", "Referer", "Authorization"},
		ExposedHeaders:   []string{"Content-Length", "Location"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	router.Use(cors.Handler(options))
	router.Use(middleware.Heartbeat("/ping"))

	router.NotFound(func(res http.ResponseWriter, req *http.Request) {
		http.Redirect(res, req, "/", http.StatusFound)
	})

	router.Post("/login", loginHandler)

	router.Group(func(r chi.Router) {
		r.Use(authenticate)

		h := asynqmon.New(asynqmon.Options{
			RootPath:     "/monitoring",
			RedisConnOpt: asynq.RedisClientOpt{Addr: env.REDIS_URL},
		})
		r.Handle(h.RootPath()+"*", h)

		fs := http.FileServer(http.Dir("asynqmon-login/.solid"))
		r.Handle("/*", fs)
	})

	return router
}
