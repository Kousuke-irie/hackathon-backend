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

// GenerateSignedUploadURL GCSへのアップロード用の署名付きURLを生成し、最終的な公開URLを返す
// フロントエンドは、この signedURL に直接ファイルを PUT します。
func GenerateSignedUploadURL(ctx context.Context, fileName string, userID uint64) (string, string, error) {
	if StorageClient == nil {
		return "", "", fmt.Errorf("gcs client is not initialized")
	}

	// 1. オブジェクト名の決定 (ユーザーIDとタイムスタンプでユニークにする)
	objectName := fmt.Sprintf("items/%d/%d-%s", userID, time.Now().Unix(), fileName)

	// 2. 署名付きURLのオプション設定
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,          // V4署名スキーム
		Method:  "PUT",                            // ファイルのアップロードにはPUTメソッドを使用
		Expires: time.Now().Add(15 * time.Minute), // 有効期限 15分
	}

	// 3. 署名付きURLの生成
	signedURL, err := StorageClient.Bucket(BucketName).SignedURL(objectName, opts)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	// 4. 最終的な公開URLを生成 (オブジェクトが公開アクセス可能になっている前提)
	imageURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", BucketName, objectName)

	return signedURL, imageURL, nil
}

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
