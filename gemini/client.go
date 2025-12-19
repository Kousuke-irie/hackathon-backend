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

// GenerateTransactionMessage はユーザーの意図に基づき、Geminiを使って適切な取引メッセージを生成します。
func GenerateTransactionMessage(ctx context.Context, userIntent string) (string, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	location := "us-central1"

	// 既存のAnalyzeImageと同様のクライアント作成ロジック
	credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var opts []option.ClientOption
	if credsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credsPath))
	}

	client, err := genai.NewClient(ctx, projectID, location, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-001") // または使用中のモデル名

	// プロンプトの構築
	prompt := fmt.Sprintf(`
	あなたはフリマアプリ「Wish」の利用者（ユーザー）に代わって、取引メッセージを執筆する代筆アシスタントです。

	【重要ルール】
	1. あなた自身がユーザーの問いかけに回答（承諾や拒否）してはいけません。
	2. ユーザーが入力した意図に基づき、相手に送るための「送信文案」のみを作成してください。
	3. 出力はメッセージ本文のみとし、説明や挨拶（「承知しました」「こちらが文案です」等）は一切含めないでください。
	
	【ユーザーの意図】
	"%s"
	
	【作成上の注意】
	- フリマアプリの慣習に沿った、丁寧で誠実な日本語（です・ます調）で作成してください。
	- 値下げ交渉の場合は、具体的な金額が含まれていればそれを反映し、無ければ「お値下げ可能でしょうか」という表現にしてください。
	- 発送遅延や質問の場合は、申し訳なさを伝えつつ明確な内容にしてください。
	`, userIntent)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	// レスポンスからテキストを抽出
	part := resp.Candidates[0].Content.Parts[0]
	if text, ok := part.(genai.Text); ok {
		return string(text), nil
	}

	return "", fmt.Errorf("unexpected response format from AI")
}

// backend/gemini/client.go

// AnalyzeUserInterest は閲覧履歴からユーザーの興味関心を分析し、検索クエリを生成します
func AnalyzeUserInterest(ctx context.Context, itemTitles []string) (string, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	location := "us-central1"

	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return "", err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-001")

	// 閲覧した商品のタイトルリストをプロンプトに組み込む
	historyStr := strings.Join(itemTitles, ", ")
	prompt := fmt.Sprintf(`
    以下の商品は、ユーザーが最近チェックした商品リストです:
    [%s]

    これらの商品から、ユーザーがいま探していそうな「中心的なキーワード」を1つだけ抽出してください。
    出力はキーワードのみ（1単語）にしてください。
    例: 「スニーカー」「キャンプ用品」「ブランドバッグ」
    `, historyStr)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("AI response is empty")
	}

	keyword, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	return strings.TrimSpace(string(keyword)), nil
}

// backend/gemini/client.go

// AnalyzeUserLikes はユーザーが「LIKE」した商品のリストから、好みの傾向を分析します
func AnalyzeUserLikes(ctx context.Context, likedTitles []string) (string, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	location := "us-central1"

	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return "", err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-001")

	historyStr := strings.Join(likedTitles, ", ")
	prompt := fmt.Sprintf(`
	以下の商品は、ユーザーが過去に「LIKE（お気に入り）」した商品のタイトルリストです:
	[%s]

	このユーザーの興味・関心を分析し、次に表示すべき商品を検索するための「具体的な検索キーワード（日本語）」を2〜3個、スペース区切りで出力してください。
	出力はキーワードのみにしてください。
	例: 「ヴィンテージ ジーンズ デニム」「キャンプ 焚き火台 アウトドア」
	`, historyStr)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("AI response empty")
	}

	keywords, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	return strings.TrimSpace(string(keywords)), nil
}

const FrontendCodeBase = `
以下はフリマアプリ「Wish」のフロントエンドの主要なソースコードの構造です。

--- App.tsx (ルーティングと全体の流れ) ---
主要なパス:
- / : 商品一覧 (ItemList)
- /items/:id : 商品詳細 (ItemDetail)
- /sell : 出品画面 (SellItem)
- /swipe : スワイプ機能 (SwipeDeck)
- /transactions/:txId : 取引画面 (TransactionScreen)
- /communities : コミュニティ一覧 (CommunityList)

--- api.tsx (API通信の定義) ---
- analyzeItemImage: AIによる画像解析API
- createItem / updateItem: 商品の作成・更新
- completePurchaseAndCreateTransaction: 購入確定
- generateAIMessage: AIによる取引メッセージ生成

--- SellItem.tsx (出品ロジック) ---
- handleAIAnalyze: Geminiによるタイトルや説明の自動入力機能。
- handleSaveLogic: 下書き(DRAFT)または販売中(ON_SALE)としての保存。

--- ItemDetail.tsx (詳細と購入) ---
- handlePurchaseClick: StripeのPaymentIntentを作成し、決済モーダルを表示。
- handleShareToCommunity: 商品をコミュニティにシェアする機能。

--- TransactionScreen.tsx (取引の進行) ---
- ステータス管理: PURCHASED -> SHIPPED -> RECEIVED -> COMPLETED。
- 役割判定: currentUser.id が seller_id か buyer_id かで操作を切り替え。
`

const AppSpecification = `
あなたは「Wish（ウィッシュ）」というWebフリマアプリの専属コンシェルジュです。
以下のアプリ構造と機能を完璧に理解し、ユーザーをサポートしてください。

【アプリの基本構成】
- React (Vite) + Go (Gin) + MySQL で構築されています。
- 認証: Firebase Authenticationを使用。Googleログインまたはメールアドレスで利用可能です。
- デザイン: 白を基調としたモダン・モノトーンなUI。アクセントカラーはピンク(#e91e63)です。

【主要機能の仕様】
1. 出品 (SellItem.tsx): 
   - カメラアイコンから開始。最大10枚の画像をアップロード可能。
   - 「AI自動入力」ボタンがあり、画像を解析してタイトル、価格、カテゴリ、タグ、説明文を自動生成します。
   - 下書き保存機能があり、後で編集・出品が可能です。

2. 商品発見 (ItemList.tsx, SwipeDeck.tsx):
   - トップ画面では、フォロー中のユーザーの新着、おすすめアカウント、閲覧履歴に基づくレコメンドが表示されます。
   - 「Discover」メニュー（スワイプ機能）では、Tinderのように商品を左右にスワイプして直感的に「いいね」を選べます。AIがLIKE履歴を分析し、好みに近い商品を優先表示します。

3. 購入と取引 (ItemDetail.tsx, TransactionScreen.tsx):
   - Stripe決済を導入。購入後は専用の「取引画面」で進行状況（発送待ち→配送中→取引完了）を確認。
   - コメント欄には「AIメッセージ作成」機能があり、値下げ交渉や質問の文案をAIが代筆します。

4. コミュニティ (CommunityBoard.tsx):
   - 共通の趣味を持つユーザーと交流可能。自分の出品物や「いいね」した商品を投稿に紐づけてシェアできます。

5. ダイレクトメッセージ (ChatScreen.tsx):
   - 出品者と購入者、またはユーザー同士でリアルタイムな1対1のチャットが可能です。

【回答の指針】
- ユーザーから「〜はどうやるの？」と聞かれたら、上記の画面遷移や機能名を出して具体的に答えてください。
- 丁寧で、親しみやすく、頼りになるショップ店員のような口調で話してください。
`

func ChatWithConcierge(ctx context.Context, userQuery string) (string, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	location := "us-central1"

	credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var opts []option.ClientOption
	if credsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credsPath))
	}

	client, err := genai.NewClient(ctx, projectID, location, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-001")

	// システム仕様、コードベース情報、ユーザーの質問を結合
	fullPrompt := fmt.Sprintf("%s\n\n【コードベース情報】\n%s\n\nユーザーからの質問: %s",
		AppSpecification, FrontendCodeBase, userQuery)

	resp, err := model.GenerateContent(ctx, genai.Text(fullPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("AIからの応答が空です")
	}

	genText, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("予期しないレスポンス形式です")
	}

	return string(genText), nil
}
