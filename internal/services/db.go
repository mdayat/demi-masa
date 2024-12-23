package services

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mdayat/demi-masa-backend/repository"
)

var (
	dbOnce  sync.Once
	dbErr   error
	db      *pgxpool.Pool
	queries *repository.Queries
)

func InitDB(ctx context.Context, DATABASE_URL string) (*pgxpool.Pool, error) {
	dbOnce.Do(func() {
		db, dbErr = pgxpool.New(ctx, DATABASE_URL)
		if dbErr != nil {
			return
		}
		queries = repository.New(db)
	})

	return db, dbErr
}

func GetDB() *pgxpool.Pool {
	return db
}

func GetQueries() *repository.Queries {
	return queries
}
