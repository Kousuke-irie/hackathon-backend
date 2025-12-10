package firebase

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// AuthClient はFirebase Authのクライアントを保持する
var AuthClient *auth.Client

// InitFirebase Firebaseの初期化を実行
func InitFirebase() error {
	opt := option.WithCredentialsFile("serviceAccountKey.json")
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return err
	}

	AuthClient, err = app.Auth(context.Background())
	if err != nil {
		return err
	}
	log.Println("Firebase Auth client initialized!")
	return nil
}
