package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/mdayat/demi-masa-be/repository"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type IDTokenClaims struct {
	Name    string
	Email   string
	Picture string
}

func loginHandler(res http.ResponseWriter, req *http.Request) {
	body := struct {
		IDToken string `json:"id_token"`
	}{}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("invalid json body")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := firebaseAuth.VerifyIDToken(context.Background(), body.IDToken)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("invalid id token")
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	var idTokenClaims IDTokenClaims
	err = mapstructure.Decode(token.Claims, &idTokenClaims)
	if err != nil {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("failed to convert map of id token claims to struct")
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	_, err = queries.GetUserByID(ctx, token.UID)
	if err != nil && errors.Is(err, pgx.ErrNoRows) == false {
		log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("failed to get user by UID")
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		user := repository.CreateUserParams{
			ID:    token.UID,
			Name:  idTokenClaims.Name,
			Email: idTokenClaims.Email,
			Role:  repository.UserRoleUser,
		}

		err = queries.CreateUser(ctx, user)
		if err != nil {
			log.Ctx(req.Context()).Error().Err(errors.Wrap(err, "loginHandler()")).Msg("failed to create new user")
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Ctx(req.Context()).Info().Msg("successfully created new user")
		res.Header().Set("Location", fmt.Sprintf("/api/users/%s", user.ID))
		res.WriteHeader(http.StatusCreated)
		return
	}

	log.Ctx(req.Context()).Info().Msg("successfully signed in")
	res.WriteHeader(http.StatusOK)
}
