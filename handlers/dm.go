package handlers

import (
	"net/http"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

// GetChatHistoryHandler 特定の相手とのチャット履歴取得
func GetChatHistoryHandler(c *gin.Context) {
	myID := c.GetHeader("X-User-ID")
	targetID := c.Param("userId")

	var messages []models.Message
	database.DBClient.
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)", myID, targetID, targetID, myID).
		Order("created_at asc").
		Find(&messages)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// GetChatThreadsHandler メッセージスレッド一覧（最新メッセージ付き）を取得
func GetChatThreadsHandler(c *gin.Context) {
	myID, _ := strconv.ParseUint(c.GetHeader("X-User-ID"), 10, 64)

	// 最新のメッセージをユーザーごとに抽出する複雑なクエリの簡略版
	var threads []struct {
		PartnerID uint64 `json:"partner_id"`
		Content   string `json:"last_message"`
		Username  string `json:"username"`
		IconURL   string `json:"icon_url"`
	}

	database.DBClient.Raw(`
		SELECT u.id as partner_id, u.username, u.icon_url, m.content as last_message
		FROM users u
		JOIN messages m ON (m.sender_id = u.id AND m.receiver_id = ?) OR (m.receiver_id = u.id AND m.sender_id = ?)
		WHERE m.id IN (
			SELECT MAX(id) FROM messages WHERE sender_id = ? OR receiver_id = ? GROUP BY LEAST(sender_id, receiver_id), GREATEST(sender_id, receiver_id)
		)
		ORDER BY m.created_at DESC`, myID, myID, myID, myID).Scan(&threads)

	c.JSON(http.StatusOK, gin.H{"threads": threads})
}
