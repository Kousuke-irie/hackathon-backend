package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourname/fleamarket-backend/database"
	"github.com/yourname/fleamarket-backend/models"
)

// UpdateUserRequest ユーザー更新用リクエスト
type UpdateUserRequest struct {
	ID       uint64 `json:"id" binding:"required"` // ユーザーID
	Username string `json:"username"`
	Bio      string `json:"bio"`
	IconURL  string `json:"icon_url"`
}

// UpdateUserHandler ユーザー情報（プロフィール）を更新
func UpdateUserHandler(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db := database.DBClient
	var user models.User

	// ユーザーの存在確認
	if err := db.First(&user, req.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// 情報の更新
	user.Username = req.Username
	user.Bio = req.Bio

	if req.IconURL != "" && req.IconURL != user.IconURL {
		user.IconURL = req.IconURL
	}

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated", "user": user})
}

// GetLikedItemsHandler ユーザーがいいねした商品一覧を取得
func GetLikedItemsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID header is required"})
		return
	}

	var items []models.Item
	db := database.DBClient

	// SQL: itemsテーブルとlikesテーブルを結合し、特定のユーザーがLIKEした商品IDをフィルタ
	if err := db.
		Joins("JOIN likes ON likes.item_id = items.id").
		Where("likes.user_id = ? AND likes.reaction = ?", userID, "LIKE").
		Where("items.status = ?", "ON_SALE"). // 販売中のもののみ
		Order("likes.created_at DESC").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch liked items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// CheckItemLikedHandler 特定の商品に対してユーザーがLike済みかチェック
func CheckItemLikedHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{"is_liked": false}) // 未ログインは当然いいねしていない
		return
	}
	itemID := c.Param("id")

	var count int64
	// likes テーブルで、user_idとitem_idが一致し、reactionが'LIKE'のレコードをカウント
	database.DBClient.Model(&models.Like{}).
		Where("user_id = ? AND item_id = ? AND reaction = ?", userID, itemID, "LIKE").
		Count(&count)

	c.JSON(http.StatusOK, gin.H{"is_liked": count > 0})
}

// GetMyPurchaseHistoryHandler 自分の購入履歴を取得
func GetMyPurchaseHistoryHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var transactions []models.Transaction
	db := database.DBClient

	// BuyerIDが自分である取引を取得し、Item情報とSeller情報をPreloadする
	if err := db.Where("buyer_id = ?", userID).
		Preload("Item").
		Preload("Item.Seller"). // 商品の出品者情報も必要なら取得
		Order("created_at DESC").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch purchase history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}
