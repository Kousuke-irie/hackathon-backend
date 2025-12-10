package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourname/fleamarket-backend/database"
	"github.com/yourname/fleamarket-backend/models"
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
}
