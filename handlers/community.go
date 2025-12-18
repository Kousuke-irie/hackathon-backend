package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
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
		CreatorID   uint64 `json:"creator_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newComm := models.Community{
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		CreatorID:   req.CreatorID,
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

	// 直近でその界隈に投稿したユーザーを取得（自分以外）
	var recentPosters []uint64
	database.DBClient.Model(&models.CommunityPost{}).
		Where("community_id = ? AND user_id != ?", communityID, req.UserID).
		Order("created_at desc").
		Limit(5).
		Pluck("user_id", &recentPosters)

	// 重複を排除して通知を送信
	sentUsers := make(map[uint64]bool)
	for _, targetID := range recentPosters {
		if !sentUsers[targetID] {
			noti := models.Notification{
				UserID:    targetID,
				Type:      "COMMUNITY",
				Content:   "あなたが参加した界隈に新しい投稿がありました",
				RelatedID: uint64(communityID),
			}
			database.DBClient.Create(&noti)
			BroadcastNotification(targetID, noti)
			sentUsers[targetID] = true
		}
	}

	c.JSON(http.StatusOK, gin.H{"post": newPost})
}

// UpdateCommunityHandler コミュニティ情報の更新
func UpdateCommunityHandler(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	var comm models.Community
	if err := database.DBClient.First(&comm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Community not found"})
		return
	}

	if fmt.Sprintf("%d", comm.CreatorID) != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "作成者のみが編集できます"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ImageURL    string `json:"image_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := database.DBClient.Model(&models.Community{}).Where("id = ?", id).Updates(models.Community{
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update community"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Community updated successfully"})
}

func DeleteCommunityHandler(c *gin.Context) {
	id := c.Param("id")
	userIDStr := c.GetHeader("X-User-ID")

	var comm models.Community
	if err := database.DBClient.First(&comm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Community not found"})
		return
	}

	if fmt.Sprintf("%d", comm.CreatorID) != userIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the creator can delete this community"})
		return
	}

	// 関連する投稿も削除する場合はトランザクションを推奨
	database.DBClient.Delete(&comm)
	c.JSON(http.StatusOK, gin.H{"message": "Community deleted"})
}
