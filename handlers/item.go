package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/gcs"
	"github.com/Kousuke-irie/hackathon-backend/gemini"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

// ItemDataRequest ★ 新規: フロントエンドの ItemData に合わせた JSON リクエストボディの型を定義
type ItemDataRequest struct {
	Title         string `json:"title" binding:"required"`
	Description   string `json:"description"`
	Price         string `json:"price" binding:"required"`
	SellerID      string `json:"seller_id" binding:"required"`
	ImageURL      string `json:"image_url"` // ★ GCSにアップロード済みのURLを受け取る
	CategoryID    string `json:"category_id" binding:"required"`
	Condition     string `json:"condition" binding:"required"`
	ShippingPayer string `json:"shipping_payer" binding:"required"`
	ShippingFee   string `json:"shipping_fee" binding:"required"`
	Status        string `json:"status" binding:"required"`
}

// CreateItemHandler 商品出品API
func CreateItemHandler(c *gin.Context) {
	var req ItemDataRequest // JSONとして受け取る
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format or missing fields"})
		return
	}

	price, err := strconv.Atoi(req.Price)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price value"})
		return
	}

	categoryID, err := strconv.ParseUint(req.CategoryID, 10, 32) // uint 型に変換
	if req.Status != "DRAFT" && (err != nil || categoryID == 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	shippingFee, _ := strconv.Atoi(req.ShippingFee)

	sellerID, err := strconv.ParseUint(req.SellerID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seller ID"})
		return
	}

	// ★ 画像URLが必須のチェック
	if req.Status != "DRAFT" && req.ImageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image URL is required for ON_SALE items"})
		return
	}

	newItem := models.Item{
		Title:         req.Title,
		Description:   req.Description,
		Price:         price,
		SellerID:      sellerID,
		ImageURL:      req.ImageURL,
		AITags:        "{}",
		Status:        req.Status,
		CategoryID:    uint(categoryID),
		Condition:     req.Condition,
		ShippingPayer: req.ShippingPayer,
		ShippingFee:   shippingFee,
	}

	if err := database.DBClient.Create(&newItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item created!", "item": newItem})
}

// AnalyzeItemHandler 画像を受け取ってAI解析結果を返す
func AnalyzeItemHandler(c *gin.Context) {
	// 1. 画像ファイルを一時保存
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image is required"})
		return
	}

	filename := filepath.Base(file.Filename)
	savePath := filepath.Join("uploads", "temp_"+filename) // 一時ファイルとして保存

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save temporary image file"})
		return
	}

	defer os.Remove(savePath)

	var allCategories []models.Category
	if err := database.DBClient.Where("parent_id IS NOT NULL").Find(&allCategories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories for AI"})
		return
	}

	validCategoryIDs := make(map[uint]bool)
	var categoriesJSON []models.Category

	for _, cat := range allCategories {
		validCategoryIDs[cat.ID] = true              // 有効なIDをマップに記録
		categoriesJSON = append(categoriesJSON, cat) // JSONプロンプト用
	}

	categoriesJSONtr, err := json.Marshal(categoriesJSON)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal categories"})
		return
	}

	// 2. Geminiで解析
	aiResult, err := gemini.AnalyzeImage(c.Request.Context(), savePath, string(categoriesJSONtr))
	if err != nil {
		fmt.Printf("AI Error: %v\n", err) // ログに出力
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI analysis failed"})
		return
	}

	if _, exists := validCategoryIDs[aiResult.CategoryID]; !exists {
		// 🚨 強制修正: IDを「その他」（ID 16）に設定し直す
		// (ID 16はご提示のデータで「その他」のトップレベルIDだが、ここでは子カテゴリの「ジャンル不明」IDを使うのが理想)
		// 暫定的に、最も具体的な子カテゴリID (例: DBに存在する最大のID) か、ユーザーが設定した「その他」のIDを使用。
		// ここでは、CategoryIDを0に設定して、フロント側で「その他」の初期値を適用させるロジックに変更します。
		aiResult.CategoryID = 0                        // 無効なIDを0に設定
		aiResult.Title = "【カテゴリ要確認】 " + aiResult.Title // タイトルにフラグを立ててユーザーに注意を促す
		fmt.Printf("Warning: AI returned invalid Category ID. Title set to: %s\n", aiResult.Title)
	}

	// 3. 結果をJSONで返す
	c.JSON(http.StatusOK, gin.H{
		"message": "AI analysis successful",
		"data":    aiResult,
	})
}

// GetItemListHandler 全ての販売中の商品を取得するAPI
func GetItemListHandler(c *gin.Context) {
	queryParam := c.Query("q")

	var items []models.Item
	db := database.DBClient

	// 自身が出品した商品を除く（スワイプと同じ条件を踏襲）
	userID := c.Query("user_id") // フロントエンドからクエリパラメータでユーザーIDを受け取る

	// 販売中で、かつ自身が出品していない商品を取得
	query := db.Where("status = ?", "ON_SALE")

	if userID != "" {
		query = query.Where("seller_id != ?", userID)
	}

	// 2. ▼ キーワード検索 (Full-Text Search / Simple LIKE) ▼
	if queryParam != "" {
		searchQuery := fmt.Sprintf("%%%s%%", queryParam)
		// title OR description で LIKE 検索
		query = query.Where("title LIKE ? OR description LIKE ?", searchQuery, searchQuery)
	}

	// 最新の20件を返す（ページネーションは一旦省略）
	if err := query.Order("created_at DESC").Limit(20).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch item list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetItemDetailHandler 商品詳細を取得（出品者情報付き）
func GetItemDetailHandler(c *gin.Context) {
	itemID := c.Param("id")

	var item models.Item

	// Preload("Seller") で、itemsテーブルのseller_idに紐づくusersテーブルの情報を一緒に取ってくる
	if err := database.DBClient.Preload("Seller").First(&item, itemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"item": item})
}

// GetMyItemsHandler ログインユーザーが出品した商品のみを取得
func GetMyItemsHandler(c *gin.Context) {
	// ユーザーID（自分の出品除外用）
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// ▼▼▼ 追加: クエリパラメータの取得 ▼▼▼
	categoryID := c.Query("category_id")
	conditionName := c.Query("condition")
	sortBy := c.Query("sort_by")       // 例: "price" or "created_at"
	sortOrder := c.Query("sort_order") // 例: "asc" or "desc"
	// ▲▲▲ 追加 ▲▲▲

	var items []models.Item
	db := database.DBClient

	query := db.Where("seller_id = ?", userID).Where("status = ?", "ON_SALE")

	// 2. ▼ 絞り込み (Filtering) ▼
	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	if conditionName != "" {
		query = query.Where("condition = ?", conditionName)
	}

	// 3. ▼ 並び替え (Sorting) ▼
	if sortBy != "" {
		order := "DESC" // デフォルトは降順
		if sortOrder == "asc" {
			order = "ASC"
		}
		// GORMで安全に並び替えを適用
		query = query.Order(fmt.Sprintf("%s %s", sortBy, order))
	} else {
		// デフォルトの並び替え
		query = query.Order("created_at DESC")
	}

	// 4. 実行
	if err := query.Limit(20).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch item list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

type UpdateItemRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Price         int    `json:"price"`
	CategoryID    uint   `json:"category_id"`
	Condition     string `json:"condition"`
	ShippingPayer string `json:"shipping_payer"`
	ShippingFee   int    `json:"shipping_fee"`
	Status        string `json:"status"` // DRAFT, ON_SALEなど
}

// UpdateItemHandler 商品情報を更新 (PUT /items/:id)
func UpdateItemHandler(c *gin.Context) {
	itemID := c.Param("id")

	var req ItemDataRequest // JSONとして受け取る
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format or missing fields"})
		return
	}

	// 2. データ型変換
	price, _ := strconv.Atoi(req.Price)
	shippingFee, _ := strconv.Atoi(req.ShippingFee)
	categoryID, _ := strconv.ParseUint(req.CategoryID, 10, 32)

	// 💡 注意: 編集時は seller_id はフォームから受け取る必要はありません

	// 3. 商品の存在確認と権限チェック
	db := database.DBClient
	var item models.Item

	if err := db.First(&item, itemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// 4. 取引中の商品編集をブロックするロジック (既存のガード)
	if item.Status != "ON_SALE" && item.Status != "DRAFT" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Item cannot be edited when it is SOLD or currently in a transaction."})
		return
	}

	// 6. GORMによる更新
	updateMap := map[string]interface{}{
		"Title":         req.Title,
		"Description":   req.Description,
		"Price":         price,
		"ImageURL":      req.ImageURL, // ★ JSONから取得したGCS URLを使用
		"CategoryID":    uint(categoryID),
		"Condition":     req.Condition,
		"ShippingPayer": req.ShippingPayer,
		"ShippingFee":   shippingFee,
		"Status":        req.Status,
	}

	if err := db.Model(&item).Updates(updateMap).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item"})
		return
	}

	// 7. 更新後のデータを返却
	db.Preload("Seller").First(&item, itemID)
	c.JSON(http.StatusOK, gin.H{"message": "Item updated", "item": item})
}

// GetMyDraftsHandler 自分の下書き商品一覧を取得
func GetMyDraftsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID header is required"})
		return
	}

	var items []models.Item
	db := database.DBClient

	// seller_id がログインユーザーIDと一致し、Statusが 'DRAFT' の商品を取得
	if err := db.Where("seller_id = ? AND status = ?", userID, "DRAFT").
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch drafts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetItemsByIdsHandler IDリストに基づいて複数の商品を取得
func GetItemsByIdsHandler(c *gin.Context) {
	// URLクエリからカンマ区切りのID文字列を取得
	idListStr := c.Query("ids")
	if idListStr == "" {
		c.JSON(http.StatusOK, gin.H{"items": []models.Item{}})
		return
	}

	// カンマ区切りの文字列をIDの配列に変換
	idStrings := strings.Split(idListStr, ",")

	// GORMで WHERE id IN (...) クエリを実行
	var items []models.Item
	if err := database.DBClient.Where("id IN (?)", idStrings).Where("status = ?", "ON_SALE").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items by IDs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetMyPurchasesInProgressHandler 自分の取引中の購入商品一覧を取得
func GetMyPurchasesInProgressHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID header is required"})
		return
	}

	var transactions []models.Transaction
	db := database.DBClient

	// buyer_id がログインユーザーIDと一致し、Statusが 'PURCHASED', 'SHIPPED', 'RECEIVED' の取引を取得
	// 'COMPLETED' (取引完了) と 'CANCELED' (キャンセル済) 以外
	inProgressStatuses := []string{"PURCHASED", "SHIPPED", "RECEIVED"}

	if err := db.
		Preload("Item").        // 関連する商品情報を取得
		Preload("Item.Seller"). // 商品の出品者情報も取得
		Where("buyer_id = ?", userID).
		Where("status IN (?)", inProgressStatuses).
		Order("created_at DESC").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch in-progress purchases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}

// GetGcsUploadUrlHandler ★ 新規: 署名付きアップロードURLを取得するハンドラ
func GetGcsUploadUrlHandler(c *gin.Context) {
	var req struct {
		FileName    string `json:"file_name" binding:"required"`
		ContentType string `json:"content_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: file_name and content_type are required"})
		return
	}

	// 認証済みユーザーIDを取得（フロントからX-User-IDが来ている前提）
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID header is required"})
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid User ID format"})
		return
	}

	// GCSの署名付きURLと最終的な画像URLを生成
	signedURL, imageURL, err := gcs.GenerateSignedUploadURL(c.Request.Context(), req.FileName, userID, req.ContentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate upload URL: %v", err)})
		return
	}

	// フロントエンドに返す
	c.JSON(http.StatusOK, gin.H{
		"uploadUrl": signedURL,
		"imageUrl":  imageURL,
	})
}

// GetMySalesInProgressHandler 自分が「販売した」取引中の商品一覧を取得 (出品者用)
func GetMySalesInProgressHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var transactions []models.Transaction
	db := database.DBClient

	// 💡 SellerID が自分で、ステータスが完了・キャンセル以外を抽出
	inProgressStatuses := []string{"PURCHASED", "SHIPPED", "RECEIVED"}

	if err := db.
		Preload("Item").
		Preload("Buyer").
		Where("seller_id = ? AND status IN (?)", userID, inProgressStatuses).
		Order("created_at DESC").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sales in progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}
