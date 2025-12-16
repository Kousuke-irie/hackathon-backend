package handlers

import (
	"fmt"
	"net/http"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

// GetSwipeItemsHandler まだスワイプしていない商品を取得
func GetSwipeItemsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID header is required"})
		return
	}

	var items []models.Item
	db := database.DBClient

	subQuery := db.Table("likes").Select("item_id").Where("user_id = ?", userID)

	if err := db.Where("id NOT IN (?)", subQuery).
		Where("status = ?", "ON_SALE").
		Where("seller_id != ?", userID).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// RecordSwipeRequest スワイプ記録用のリクエストボディ
type RecordSwipeRequest struct {
	UserID   uint64 `json:"user_id"`
	ItemID   uint64 `json:"item_id"`
	Reaction string `json:"reaction"` // "LIKE" or "NOPE"
}

// RecordSwipeHandler スワイプ結果(Like/Nope)を保存
func RecordSwipeHandler(c *gin.Context) {
	var req RecordSwipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newLike := models.Like{
		UserID:   req.UserID,
		ItemID:   req.ItemID,
		Reaction: req.Reaction,
	}

	if err := database.DBClient.Create(&newLike).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record swipe"})
		return
	}

	if req.Reaction == "LIKE" {
		var item models.Item
		database.DBClient.First(&item, req.ItemID)

		// 相手に通知
		noti := models.Notification{
			UserID:    item.SellerID,
			Type:      "LIKE",
			Content:   fmt.Sprintf("あなたの出品した「%s」にいいね！がつきました", item.Title),
			RelatedID: item.ID,
		}
		database.DBClient.Create(&noti)
		BroadcastNotification(item.SellerID, noti)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Swipe recorded"})
}
