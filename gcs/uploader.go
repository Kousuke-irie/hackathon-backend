package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	// 署名に必要なサービスアカウントキーの情報をロード
	keyFilePath := "serviceAccountKey.json" // Dockerfileでこの名前でコピーされている
	keyData, err := os.ReadFile(keyFilePath)
	if err != nil {
		// サービスアカウントキーファイルの読み込み失敗
		return "", "", fmt.Errorf("failed to read service account key file: %w", err)
	}

	var saKey ServiceAccountKey
	if err := json.Unmarshal(keyData, &saKey); err != nil {
		// JSONパース失敗
		return "", "", fmt.Errorf("failed to parse service account key file: %w", err)
	}

	// 1. オブジェクト名の決定 (ユーザーIDとタイムスタンプでユニークにする)
	objectName := fmt.Sprintf("items/%d/%d-%s", userID, time.Now().Unix(), fileName)

	// 2. 署名付きURLのオプション設定
	opts := &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,          // V4署名スキーム
		Method:         "PUT",                            // ファイルのアップロードにはPUTメソッドを使用
		Expires:        time.Now().Add(15 * time.Minute), // 有効期限 15分
		GoogleAccessID: saKey.ClientEmail,
		PrivateKey:     []byte(saKey.PrivateKey),
		ContentType:    contentType,
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
