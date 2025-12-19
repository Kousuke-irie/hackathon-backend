package handlers

import (
	"fmt"
	"net/http"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/gemini"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

// GetCategoriesHandler カテゴリ一覧を取得
func GetCategoriesHandler(c *gin.Context) {
	var categories []models.Category
	if err := database.DBClient.Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// GetCategoryTreeHandler 階層化されたカテゴリツリー全体を取得
func GetCategoryTreeHandler(c *gin.Context) {
	var topCategories []models.Category
	db := database.DBClient

	// 1. トップレベルのカテゴリを取得 (ParentIDがNULLのもの)
	// 2. Preload("Children") を使用し、リレーションに基づいて子カテゴリも自動的にロード
	if err := db.Where("parent_id IS NULL").Preload("Children").Find(&topCategories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch category tree"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": topCategories})
}

// GetConditionsHandler 商品状態一覧を取得
func GetConditionsHandler(c *gin.Context) {
	var conditions []models.ProductCondition
	// Rank順に並べ替える
	if err := database.DBClient.Find(&conditions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch conditions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"conditions": conditions})
}

func AIChatConciergeHandler(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "質問内容を入力してください"})
		return
	}

	// backend/gemini/client.go で定義した関数を呼び出す
	answer, err := gemini.ChatWithConcierge(c.Request.Context(), req.Query)
	if err != nil {
		fmt.Printf("AI Concierge Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AIコンシェルジュが一時的に応答できません"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"answer": answer})
}
