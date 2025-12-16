package gcs

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
)

type ServiceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

const BucketName = "hackathon-kousuke"

// GenerateSignedUploadURL GCSへのアップロード用の署名付きURLを生成し、最終的な公開URLを返す
// フロントエンドは、この signedURL に直接ファイルを PUT します。
func GenerateSignedUploadURL(ctx context.Context, fileName string, userID uint64, contentType string) (string, string, error) {
	if StorageClient == nil {
		return "", "", fmt.Errorf("gcs client is not initialized")
	}

	// 1. オブジェクト名の決定
	objectName := fmt.Sprintf("items/%d/%d-%s", userID, time.Now().Unix(), fileName)

	// 2. ブラウザが送るヘッダーを署名対象にする（403エラー対策）
	signedHeaders := []string{
		"Content-Length",
	}

	// 3. 署名オプションの設定（秘密鍵とGoogleAccessIDを指定しない）
	opts := &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		Expires:     time.Now().Add(15 * time.Minute),
		ContentType: contentType,
		Headers:     signedHeaders,
		// ★ ここがポイント: GoogleAccessID と PrivateKey を空にする
		// これにより、SDKは環境（Cloud Run等）のデフォルトサービスアカウントを使用して署名を試みます
	}

	// 4. 署名付きURLの生成
	signedURL, err := StorageClient.Bucket(BucketName).SignedURL(objectName, opts)
	if err != nil {
		return "", "", fmt.Errorf("署名付きURLの生成に失敗しました（IAM権限を確認してください）: %w", err)
	}

	imageURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", BucketName, objectName)
	return signedURL, imageURL, nil
}
