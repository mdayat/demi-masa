package httpservice

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/odemimasa/backend/repository"
	"github.com/rs/zerolog/log"
)

type taskRespBody struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Checked     bool   `json:"checked"`
}

func getTasksHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	userID := fmt.Sprintf("%s", ctx.Value("userID"))

	tasks, err := queries.GetTasksByUserID(ctx, userID)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to get tasks by user id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	respBody := make([]taskRespBody, len(tasks))
	for i, task := range tasks {
		taskID, err := task.ID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to get task UUID from pgtype.UUID")
			http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		respBody[i] = taskRespBody{
			ID:          fmt.Sprintf("%s", taskID),
			Name:        task.Name,
			Description: task.Description,
			Checked:     task.Checked,
		}
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusOK, Data: &respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}

func createTaskHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var body struct {
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID := fmt.Sprintf("%s", ctx.Value("userID"))
	task, err := queries.CreateTask(ctx, repository.CreateTaskParams{
		UserID:      userID,
		Name:        body.Name,
		Description: body.Description,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to create task")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	taskID, err := task.ID.Value()
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to get task UUID from pgtype.UUID")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	respBody := taskRespBody{
		ID:          fmt.Sprintf("%s", taskID),
		Name:        task.Name,
		Description: task.Description,
		Checked:     task.Checked,
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusCreated, Data: respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Location", fmt.Sprintf("/tasks/%s", taskID))
	logWithCtx.Info().Int("status_code", http.StatusCreated).Dur("response_time", time.Since(start)).Msg("request completed")
}

func updateTaskHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()
	var body struct {
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
		Checked     bool   `json:"checked"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusBadRequest).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	taskID := chi.URLParam(req, "taskID")
	taskIDBytes, err := uuid.Parse(taskID)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to parse task uuid string to bytes")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = queries.UpdateTaskByID(ctx, repository.UpdateTaskByIDParams{
		ID:          pgtype.UUID{Bytes: taskIDBytes, Valid: true},
		Name:        body.Name,
		Description: body.Description,
		Checked:     body.Checked,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to update task by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}

func deleteTaskHandler(res http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logWithCtx := log.Ctx(ctx).With().Logger()

	taskID := chi.URLParam(req, "taskID")
	taskIDBytes, err := uuid.Parse(taskID)
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to parse task uuid string to bytes")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = queries.DeleteTaskByID(ctx, pgtype.UUID{Bytes: taskIDBytes, Valid: true})
	if err != nil {
		logWithCtx.Error().Err(err).Int("status_code", http.StatusInternalServerError).Msg("failed to delete task by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Int("status_code", http.StatusOK).Dur("response_time", time.Since(start)).Msg("request completed")
}
