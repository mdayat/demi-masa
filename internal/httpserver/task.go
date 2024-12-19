package httpserver

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/rs/zerolog/log"
)

type taskRespBody struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Checked     bool   `json:"checked"`
}

func getTasksHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	userID := fmt.Sprintf("%s", req.Context().Value("userID"))

	tasks, err := queries.GetTasksByUserID(req.Context(), userID)
	if err != nil {
		logWithCtx.Error().Err(err).Str("user_id", userID).Msg("failed to get tasks by user id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Str("user_id", userID).Msg("successfully get tasks by user id")

	respBody := make([]taskRespBody, len(tasks))
	for i, task := range tasks {
		taskID, err := task.ID.Value()
		if err != nil {
			logWithCtx.Error().Err(err).Msg("failed to get task UUID from pgtype.UUID")
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
		logWithCtx.Error().Err(err).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully sent successful response body")
}

func createTaskHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var body struct {
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	logWithCtx.Info().Msg("successfully decoded and validated request body")

	userID := fmt.Sprintf("%s", req.Context().Value("userID"))
	task, err := queries.CreateTask(req.Context(), repository.CreateTaskParams{
		UserID:      userID,
		Name:        body.Name,
		Description: body.Description,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to create task")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully created task")

	taskID, err := task.ID.Value()
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to get task UUID from pgtype.UUID")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully get task UUID from pgtype.UUID")

	respBody := taskRespBody{
		ID:          fmt.Sprintf("%s", taskID),
		Name:        task.Name,
		Description: task.Description,
		Checked:     task.Checked,
	}

	err = sendJSONSuccessResponse(res, successResponseParams{StatusCode: http.StatusCreated, Data: respBody})
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to send successful response body")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	logWithCtx.Info().Msg("successfully sent successful response body")
	res.Header().Set("Location", fmt.Sprintf("/tasks/%s", taskID))
}

func updateTaskHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	var body struct {
		Name        string `json:"name" validate:"required"`
		Description string `json:"description" validate:"required"`
		Checked     bool   `json:"checked"`
	}

	err := decodeAndValidateJSONBody(req, &body)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("invalid request body")
		http.Error(res, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	logWithCtx.Info().Msg("successfully decoded and validated request body")

	taskID := chi.URLParam(req, "taskID")
	taskIDBytes, err := uuid.Parse(taskID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to parse task uuid string to bytes")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully parsed task uuid string to bytes")

	err = queries.UpdateTaskByID(req.Context(), repository.UpdateTaskByIDParams{
		ID:          pgtype.UUID{Bytes: taskIDBytes, Valid: true},
		Name:        body.Name,
		Description: body.Description,
		Checked:     body.Checked,
	})

	if err != nil {
		logWithCtx.Error().Err(err).Str("task_id", taskID).Msg("failed to update task by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Str("task_id", taskID).Msg("successfully updated task by id")
}

func deleteTaskHandler(res http.ResponseWriter, req *http.Request) {
	logWithCtx := log.Ctx(req.Context()).With().Logger()
	taskID := chi.URLParam(req, "taskID")
	taskIDBytes, err := uuid.Parse(taskID)
	if err != nil {
		logWithCtx.Error().Err(err).Msg("failed to parse task uuid string to bytes")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Msg("successfully parsed task uuid string to bytes")

	err = queries.DeleteTaskByID(req.Context(), pgtype.UUID{Bytes: taskIDBytes, Valid: true})
	if err != nil {
		logWithCtx.Error().Err(err).Str("task_id", taskID).Msg("failed to delete task by id")
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logWithCtx.Info().Str("task_id", taskID).Msg("successfully deleted task by id")
}
