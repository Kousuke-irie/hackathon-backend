package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/firebase"
	"github.com/Kousuke-irie/hackathon-backend/routes"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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
		//c.JSON(http.StatusOK, gin.H{"message": "API is running"})
		c.JSON(http.StatusOK, gin.H{"message": "Connectivity Test Succeeded"})
	})

	routes.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Println("Server starting on " + port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
