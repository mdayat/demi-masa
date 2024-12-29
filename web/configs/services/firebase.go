package services

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

var (
	FirebaseApp  *firebase.App
	FirebaseAuth *auth.Client
)

func InitFirebase(ctx context.Context) error {
	FirebaseApp, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return err
	}

	FirebaseAuth, err = FirebaseApp.Auth(ctx)
	if err != nil {
		return err
	}

	return nil
}
