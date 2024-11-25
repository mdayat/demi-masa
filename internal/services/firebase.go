package services

import (
	"context"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

var (
	firebaseOnce sync.Once
	firebaseErr  error
	firebaseApp  *firebase.App
	firebaseAuth *auth.Client
)

func InitFirebase(ctx context.Context) error {
	firebaseOnce.Do(func() {
		firebaseApp, firebaseErr = firebase.NewApp(ctx, nil)
		if firebaseErr != nil {
			return
		}

		firebaseAuth, firebaseErr = firebaseApp.Auth(ctx)
		if firebaseErr != nil {
			return
		}
	})

	return firebaseErr
}

func GetFirebaseApp() *firebase.App {
	return firebaseApp
}

func GetFirebaseAuth() *auth.Client {
	return firebaseAuth
}
