package gcs

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"cloud.google.com/go/storage"
)

const BucketName = "hackathon-kousuke"

// UploadFile はファイルをGCSにアップロードし、公開URLを返す
func UploadFile(ctx context.Context, file *multipart.FileHeader, userID uint64) (string, error) {
	if StorageClient == nil {
		return "", fmt.Errorf("gcs client is not initialized. Cannot upload file")
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// GCSでのファイルパス/オブジェクト名を設定
	objectName := fmt.Sprintf("items/%d/%d-%s", userID, time.Now().Unix(), file.Filename)

	// バケットとオブジェクトを参照
	wc := StorageClient.Bucket(BucketName).Object(objectName).NewWriter(ctx)

	// GCSで公開閲覧可能にするための設定
	wc.ContentType = file.Header.Get("Content-Type")
	wc.ACL = []storage.ACLRule{
		{Entity: storage.AllUsers, Role: storage.RoleReader},
	}

	if _, err = io.Copy(wc, src); err != nil {
		return "", fmt.Errorf("failed to copy file to GCS: %w", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	// 公開URLを返す
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", BucketName, objectName), nil
}
