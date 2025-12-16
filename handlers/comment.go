package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

// GetCommentsHandler 商品のコメント一覧を取得
func GetCommentsHandler(c *gin.Context) {
	itemID := c.Param("id")

	var comments []models.Comment
	// 投稿したユーザーの情報も一緒に取得 (Preload)
	if err := database.DBClient.Preload("User").Where("item_id = ?", itemID).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// PostCommentHandler コメントを投稿
func PostCommentHandler(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, _ := strconv.ParseUint(itemIDStr, 10, 64)

	var req struct {
		UserID  uint64 `json:"user_id"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newComment := models.Comment{
		ItemID:  itemID,
		UserID:  req.UserID,
		Content: req.Content,
	}

	if err := database.DBClient.Create(&newComment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post comment"})
		return
	}

	// 作成したコメントにユーザー情報を紐付けて返す（フロントでの表示用）
	database.DBClient.Preload("User").First(&newComment, newComment.ID)

	c.JSON(http.StatusOK, gin.H{"comment": newComment})

	// 通知の作成と送信
	var item models.Item
	database.DBClient.First(&item, newComment.ItemID)

	// 自分の商品へのコメントでない場合のみ通知
	if item.SellerID != req.UserID {
		noti := models.Notification{
			UserID:    item.SellerID,
			Type:      "COMMENT",
			Content:   fmt.Sprintf("%sさんがあなたの商品にコメントしました", newComment.User.Username),
			RelatedID: item.ID,
		}
		database.DBClient.Create(&noti)

		// WebSocket経由でリアルタイム送信
		BroadcastNotification(item.SellerID, noti)
	}
}
