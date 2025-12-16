package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ClientManager は接続中のWebSocketクライアントを管理します
type ClientManager struct {
	clients map[uint64]*websocket.Conn
	mu      sync.Mutex
}

var Manager = ClientManager{
	clients: make(map[uint64]*websocket.Conn),
}

// WSNotificationHandler WebSocket接続の確立
func WSNotificationHandler(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	// 文字列からIDへ変換 (エラーチェックは簡略化)
	var userID uint64
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket Upgrade Error: %v", err)
		return
	}

	Manager.mu.Lock()
	Manager.clients[userID] = conn
	Manager.mu.Unlock()

	log.Printf("User %d connected via WebSocket", userID)

	// 接続維持のためのダミーループ
	defer func() {
		Manager.mu.Lock()
		delete(Manager.clients, userID)
		Manager.mu.Unlock()
		conn.Close()
		log.Printf("User %d disconnected", userID)
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// BroadcastNotification 特定のユーザーに通知をリアルタイム送信
func BroadcastNotification(userID uint64, notification models.Notification) {
	Manager.mu.Lock()
	conn, ok := Manager.clients[userID]
	Manager.mu.Unlock()

	if ok {
		if err := conn.WriteJSON(notification); err != nil {
			log.Printf("Failed to send WS message to user %d: %v", userID, err)
			conn.Close()
			Manager.mu.Lock()
			delete(Manager.clients, userID)
			Manager.mu.Unlock()
		}
	}
}
