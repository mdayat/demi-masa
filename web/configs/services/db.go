package services

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mdayat/demi-masa/web/repository"
)

var (
	DB      *pgxpool.Pool
	Queries *repository.Queries
)

func InitDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	var err error
	DB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	Queries = repository.New(DB)

	return DB, err
}
