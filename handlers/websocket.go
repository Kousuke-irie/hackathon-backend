package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ClientManager struct {
	clients map[uint64]*websocket.Conn
	mu      sync.Mutex
}

var Manager = ClientManager{
	clients: make(map[uint64]*websocket.Conn),
}

func WSNotificationHandler(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	var userID uint64
	fmt.Sscanf(userIDStr, "%d", &userID)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket Upgrade Error: %v", err)
		return
	}

	Manager.mu.Lock()
	Manager.clients[userID] = conn
	Manager.mu.Unlock()

	defer func() {
		Manager.mu.Lock()
		delete(Manager.clients, userID)
		Manager.mu.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// BroadcastChatMessage メッセージを特定の相手にリアルタイム転送
func BroadcastChatMessage(receiverID uint64, msg models.Message) {
	Manager.mu.Lock()
	conn, ok := Manager.clients[receiverID]
	Manager.mu.Unlock()

	if ok {
		// フロントエンドが識別しやすいように型を付けて送信
		payload := gin.H{
			"type":    "CHAT_MESSAGE",
			"message": msg,
		}
		if err := conn.WriteJSON(payload); err != nil {
			conn.Close()
			Manager.mu.Lock()
			delete(Manager.clients, receiverID)
			Manager.mu.Unlock()
		}
	}
}

// PostMessageHandler メッセージをDB保存し、WSで送信
func PostMessageHandler(c *gin.Context) {
	var msg models.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	senderIDStr := c.GetHeader("X-User-ID")
	senderID, _ := strconv.ParseUint(senderIDStr, 10, 64)
	msg.SenderID = senderID

	if err := database.DBClient.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}

	// 相手がオンラインならWSで即時送信
	BroadcastChatMessage(msg.ReceiverID, msg)

	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func BroadcastNotification(userID uint64, notification models.Notification) {
	Manager.mu.Lock()
	conn, ok := Manager.clients[userID]
	Manager.mu.Unlock()

	if ok {
		if err := conn.WriteJSON(notification); err != nil {
			conn.Close()
			Manager.mu.Lock()
			delete(Manager.clients, userID)
			Manager.mu.Unlock()
		}
	}
}
