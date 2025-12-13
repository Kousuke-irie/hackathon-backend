package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/api/option"
)

// AIResponse Geminiが返すデータの構造
type AIResponse struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       int      `json:"price"`
	Tags        []string `json:"tags"`
	CategoryID  uint     `json:"category_id"`
}

// AnalyzeImage 画像をGeminiに投げて解析結果を返す
func AnalyzeImage(ctx context.Context, imagePath string, categoriesJSON string) (*AIResponse, error) {
	projectID := os.Getenv("GCP_PROJECT_ID") // 後で環境変数に追加します
	location := "us-central1"                // Geminiが使えるリージョン

	credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var opts []option.ClientOption

	if credsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credsPath))
	} else {
		log.Println("WARNING: GOOGLE_APPLICATION_CREDENTIALS is not set for Gemini. Trying default authentication...")
	}

	client, err := genai.NewClient(ctx, projectID, location, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-001")

	// 期待するレスポンスのフォーマットを強制する設定
	model.ResponseMIMEType = "application/json"

	// 画像ファイルの読み込み
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	// プロンプト（命令文）
	prompt := fmt.Sprintf(`
	あなたはフリマアプリの出品アシスタントです。
	アップロードされた画像を解析し、以下の情報をJSON形式で出力してください。

	- title: 魅力的で簡潔な商品名 (30文字以内)
	- description: 購買意欲をそそる商品説明 (100文字〜200文字程度)。状態や色、用途などに触れること。
	- price: 画像から推測される適正な中古販売価格（日本円、整数のみ）。
	- tags: 検索されやすそうなタグの配列 (5つ程度)。
	- category_id: 以下の利用可能なカテゴリリストから、**IDのみを厳密に**選択してください。
	利用可能なカテゴリリスト:
	%s

	**【重要】選択肢にないID (例: 1〜16のトップレベルID) は絶対に使用しないでください。**

	出力例:
	{
		"title": "NIKE エアジョーダン スニーカー 27cm",
		"description": "数回使用した程度の美品です。人気の赤黒カラー...",
		"price": 8500,
		"tags": ["スニーカー", "NIKE", "靴", "メンズ", "エアジョーダン"]
		"category_id": 105
	}
	`, categoriesJSON)

	// AIへ送信
	resp, err := model.GenerateContent(ctx,
		genai.Text(prompt),
		genai.ImageData("jpeg", imgData), // 拡張子は簡易的にjpegとしていますがpngでも通ります
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	// レスポンス（JSON文字列）を取得
	jsonStr, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	// JSONを構造体に変換
	var result AIResponse
	// マークダウンのコードブロックが含まれる場合の除去処理
	cleanJSON := strings.TrimPrefix(string(jsonStr), "```json")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &result, nil
}
