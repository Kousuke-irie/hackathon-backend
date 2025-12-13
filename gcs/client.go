package gcs

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

var StorageClient *storage.Client

func InitStorageClient() error {
	ctx := context.Background()

	credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	// デプロイ環境や環境変数が設定されていない場合はデフォルトの認証メカニズムにフォールバック
	if credsPath == "" {
		log.Println("WARNING: GOOGLE_APPLICATION_CREDENTIALS is not set. Trying default client initialization...")
		client, err := storage.NewClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to create default GCS client: %w", err)
		}
		StorageClient = client
		log.Println("GCS client initialized with default credentials!")
		return nil
	}

	opt := option.WithCredentialsFile(credsPath)
	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	StorageClient = client
	log.Println("GCS client initialized!")
	return nil
}
