package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/firebase"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 1. Firebaseでトークンを検証
	token, err := firebase.AuthClient.VerifyIDToken(context.Background(), req.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// 2. トークンから情報取得
	firebaseUID := token.UID
	email, _ := token.Claims["email"].(string)

	// 💡 Googleログイン以外では名前や画像が空になるため、デフォルト値を設定
	name, _ := token.Claims["name"].(string)
	if name == "" && email != "" {
		name = strings.Split(email, "@")[0] // メアドの@前を仮の名前にする
	}

	picture, _ := token.Claims["picture"].(string)
	if picture == "" {
		picture = "https://www.gravatar.com/avatar/00000000000000000000000000000000?d=mp&f=y" // デフォルトアイコン
	}

	// 3. Upsert ロジック
	var user models.User
	db := database.DBClient

	result := db.Where("firebase_uid = ?", firebaseUID).First(&user)

	if result.Error == nil {
		// 既存ユーザーの更新（ログインごとに最新情報を反映）
		user.Email = email
		if user.Username == "" {
			user.Username = name
		} // 名前がなければ更新
		db.Save(&user)
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// メアドで既存ユーザーを検索（UID未紐付け対策）
		emailResult := db.Where("email = ?", email).First(&user)

		if emailResult.Error == nil {
			user.FirebaseUID = firebaseUID
			user.Email = email
			if user.Username == "" {
				user.Username = name
			}
			db.Save(&user)
		} else if errors.Is(emailResult.Error, gorm.ErrRecordNotFound) {
			// 完全新規作成
			user = models.User{
				FirebaseUID: firebaseUID,
				Email:       email,
				Username:    name,
				IconURL:     picture,
			}
			if err := db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create new user"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error during email check"})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error during UID check"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    user,
	})
}
