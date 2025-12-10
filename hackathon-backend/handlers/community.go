package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourname/fleamarket-backend/database"
	"github.com/yourname/fleamarket-backend/models"
)

// GetCommunitiesHandler 全てのコミュニティを取得
func GetCommunitiesHandler(c *gin.Context) {
	var communities []models.Community
	if err := database.DBClient.Find(&communities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch communities"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"communities": communities})
}

// CreateCommunityHandler コミュニティを作成（今回は簡易的に誰でも作れるようにします）
func CreateCommunityHandler(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ImageURL    string `json:"image_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newComm := models.Community{
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
	}

	if err := database.DBClient.Create(&newComm).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create community"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"community": newComm})
}

// GetCommunityPostsHandler コミュニティの投稿一覧を取得
func GetCommunityPostsHandler(c *gin.Context) {
	communityID := c.Param("id")

	var posts []models.CommunityPost
	// 投稿者情報(User)と、シェアされた商品情報(RelatedItem)を一緒に取得
	if err := database.DBClient.
		Preload("User").
		Preload("RelatedItem").
		Where("community_id = ?", communityID).
		Order("created_at desc"). // 新しい順
		Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// PostToCommunityHandler コミュニティに投稿
func PostToCommunityHandler(c *gin.Context) {
	communityIDStr := c.Param("id")
	communityID, _ := strconv.ParseUint(communityIDStr, 10, 64)

	var req struct {
		UserID        uint64  `json:"user_id"`
		Content       string  `json:"content"`
		RelatedItemID *uint64 `json:"related_item_id"` // 商品ID（任意）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newPost := models.CommunityPost{
		CommunityID:   communityID,
		UserID:        req.UserID,
		Content:       req.Content,
		RelatedItemID: req.RelatedItemID,
	}

	if err := database.DBClient.Create(&newPost).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post"})
		return
	}

	// レスポンス用に紐付け情報を再取得
	database.DBClient.Preload("User").Preload("RelatedItem").First(&newPost, newPost.ID)

	c.JSON(http.StatusOK, gin.H{"post": newPost})
}
