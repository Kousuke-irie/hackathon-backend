package main

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/firebase"
	"github.com/Kousuke-irie/hackathon-backend/routes"
)

func main() {
	// 1. 接続と初期化
	if err := database.InitDB(); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	if err := firebase.InitFirebase(); err != nil {
		log.Fatalf("Firebase initialization failed: %v", err)
	}

	// 2. ルーティング設定
	r := gin.Default()

	// CORS設定
	config := cors.DefaultConfig()
	/*	config.AllowOrigins = []string{
			"https://hackathon-frontend-5xp7.vercel.app",
			"https://*.vercel.app",
		}
		config.AllowWildcard = true
	*/
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-User-ID"}
	config.AllowCredentials = false
	r.Use(cors.New(config))

	// 静的ファイル（画像）の配信
	r.Static("/uploads", "./uploads")

	// 疎通確認用
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "API is running"})
	})

	routes.SetupRoutes(r)

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil { // 👈 エラーチェックを追加
		log.Fatalf("Server failed to run: %v", err)
	}
}
