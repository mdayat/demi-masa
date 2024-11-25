package workerserver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/mdayat/demi-masa-be/internal/task"
	"github.com/mdayat/demi-masa-be/repository"
)

func handleCleanupOrder(ctx context.Context, asynqTask *asynq.Task) error {
	logWithCtx := log.Ctx(ctx).With().Logger()
	var payload task.OrderTaskPayload

	if err := json.Unmarshal(asynqTask.Payload(), &payload); err != nil {
		logWithCtx.Error().Err(err).Msg("failed to unmarshal cleanup order task payload")
		return err
	}

	uuidBytes, err := uuid.Parse(payload.OrderID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to parse order id to uuid bytes")
		return err
	}

	order, err := queries.GetOrderByID(ctx, pgtype.UUID{Bytes: uuidBytes, Valid: true})
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get order by id")
		return err
	}

	if order.PaidAt.Valid && order.PaymentStatus == repository.PaymentStatusPaid {
		return nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to start db transaction to cleanup order and/or increment coupon quota")
		return err
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)
	err = qtx.DeleteOrderByID(ctx, order.ID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to delete order by id")
		return err
	}

	if order.CouponCode.Valid {
		err = qtx.IncrementCouponQuota(ctx, order.CouponCode.String)
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to increment coupon quota")
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to commit db transaction to cleanup order and/or increment coupon quota")
		return err
	}

	return nil
}
