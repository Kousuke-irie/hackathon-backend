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
	if req.Status != "DRAFT" && (req.ImageURL == "" || req.ImageURL == "[]") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one image is required for ON_SALE items"})
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

	// 💡 デバッグ用: AIが何を返したかログに出す
	fmt.Printf("AI returned CategoryID: %d for Title: %s\n", aiResult.CategoryID, aiResult.Title)

	// カテゴリIDの存在確認
	var count int64
	database.DBClient.Model(&models.Category{}).Where("id = ?", aiResult.CategoryID).Count(&count)

	if count == 0 {
		// AIが全く存在しないIDを返した場合のみ 0 にする
		fmt.Printf("Warning: AI returned non-existent Category ID: %d\n", aiResult.CategoryID)
		aiResult.CategoryID = 0
	}

	// 3. 結果をJSONで返す
	c.JSON(http.StatusOK, gin.H{
		"message": "AI analysis successful",
		"data":    aiResult,
	})
}

func GetItemListHandler(c *gin.Context) {
	queryParam := c.Query("q")
	categoryIDStr := c.Query("category_id") // フロントから渡されるカテゴリID
	conditionName := c.Query("condition")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")
	userID := c.Query("user_id")

	var items []models.Item
	db := database.DBClient

	query := db.Where("status = ?", "ON_SALE")

	if userID != "" {
		query = query.Where("seller_id != ?", userID)
	}

	// 💡 カテゴリ絞り込みの強化
	if categoryIDStr != "" {
		catID, _ := strconv.ParseUint(categoryIDStr, 10, 64)
		// 子カテゴリのIDリストを取得
		var subCategoryIDs []uint
		database.DBClient.Model(&models.Category{}).
			Where("id = ? OR parent_id = ?", catID, catID).
			Pluck("id", &subCategoryIDs)

		query = query.Where("category_id IN (?)", subCategoryIDs)
	}

	if conditionName != "" {
		query = query.Where("condition = ?", conditionName)
	}

	if queryParam != "" {
		searchQuery := fmt.Sprintf("%%%s%%", queryParam)
		query = query.Where("title LIKE ? OR description LIKE ?", searchQuery, searchQuery)
	}

	// 並び替えの適用
	order := "DESC"
	if sortOrder == "asc" {
		order = "ASC"
	}
	sortCol := "created_at"
	if sortBy == "price" {
		sortCol = "price"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortCol, order))

	if err := query.Preload("Seller").Limit(40).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
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
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// クエリパラメータからステータスを取得 (デフォルトは ON_SALE)
	statusFilter := c.Query("status")
	if statusFilter == "" {
		statusFilter = "ON_SALE"
	}

	var items []models.Item
	db := database.DBClient

	// ステータスでフィルタリングするようにクエリを構成
	query := db.Where("seller_id = ? AND status = ?", userID, statusFilter)

	// 並び替えなどは既存のロジックを維持
	query = query.Order("created_at DESC")

	if err := query.Limit(40).Find(&items).Error; err != nil {
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
	userID := c.GetHeader("X-User-ID")

	var req ItemDataRequest // JSONとして受け取る
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format or missing fields"})
		return
	}

	// 2. データ型変換
	price, _ := strconv.Atoi(req.Price)
	shippingFee, _ := strconv.Atoi(req.ShippingFee)
	categoryID, _ := strconv.ParseUint(req.CategoryID, 10, 32)

	// 3. 商品の存在確認と権限チェック
	db := database.DBClient
	var item models.Item

	if err := db.First(&item, itemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// 💡 権限チェック: 出品者本人以外は編集不可
	if strconv.FormatUint(item.SellerID, 10) != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to edit this item"})
		return
	}

	// 💡 取引中(SOLD)以外は編集可能にする
	if item.Status == "SOLD" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sold items cannot be edited"})
		return
	}

	// 6. GORMによる更新
	updateMap := map[string]interface{}{
		"Title":         req.Title,
		"Description":   req.Description,
		"Price":         price,
		"image_url":     req.ImageURL, // ★ JSONから取得したGCS URLを使用
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

// GetMySalesHistoryHandler 自分が「販売した」完了済みの取引一覧を取得 (出品者用)
func GetMySalesHistoryHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var transactions []models.Transaction
	db := database.DBClient

	// 💡 ステータスが完了(COMPLETED)または受取済(RECEIVED)のものを抽出
	completedStatuses := []string{"COMPLETED", "RECEIVED"}

	if err := db.
		Preload("Item").
		Preload("Buyer").
		Where("seller_id = ? AND status IN (?)", userID, completedStatuses).
		Order("created_at DESC").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sales history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}

// GetFollowingItemsHandler フォロー中ユーザーの出品を取得
func GetFollowingItemsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	var items []models.Item
	// サブクエリでフォロー中のIDを抽出し、それらの最新出品を取得
	database.DBClient.
		Joins("JOIN follows ON follows.following_id = items.seller_id").
		Where("follows.follower_id = ? AND items.status = ?", userID, "ON_SALE").
		Order("items.created_at DESC").Limit(10).Find(&items)
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetCategoryRecommendationsHandler 最近の閲覧・購入カテゴリからおすすめを取得
func GetCategoryRecommendationsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	// 簡略化例: 直近の「購入」カテゴリを取得し、そのカテゴリから自分以外の商品を出す
	var lastCategoryID uint
	database.DBClient.Model(&models.Transaction{}).
		Joins("JOIN items ON items.id = transactions.item_id").
		Where("transactions.buyer_id = ?", userID).
		Order("transactions.created_at DESC").Limit(1).Pluck("items.category_id", &lastCategoryID)

	var items []models.Item
	database.DBClient.Where("category_id = ? AND seller_id != ? AND status = ?", lastCategoryID, userID, "ON_SALE").
		Limit(10).Find(&items)
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetRecommendedUsersHandler おすすめのアカウント（共通のカテゴリを出品している人など）
func GetRecommendedUsersHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	var users []models.User
	// 実装例: まだフォローしていない、かつ出品数が多いユーザーを推奨
	database.DBClient.Where("id != ? AND id NOT IN (SELECT following_id FROM follows WHERE follower_id = ?)", userID, userID).
		Order("follower_count DESC").Limit(8).Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users})
}
