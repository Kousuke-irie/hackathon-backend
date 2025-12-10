package handlers

import (
	"context"
	"errors" // ★ 追加が必要
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/firebase"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"gorm.io/gorm" // gorm.ErrRecordNotFound を使うために必要
)

// LoginRequest フロントエンドから送られてくるデータ型（再定義）
type LoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// LoginHandler ログインおよび新規ユーザー登録 (Upsert) を処理
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
	email := token.Claims["email"].(string)
	name, _ := token.Claims["name"].(string)
	picture, _ := token.Claims["picture"].(string)

	// 3. 堅牢な Upsert ロジック
	var user models.User
	db := database.DBClient

	// A. まず Firebase UID で検索
	result := db.Where("firebase_uid = ?", firebaseUID).First(&user)

	if result.Error == nil {
		// B1. UIDで見つかった場合 -> 既存ユーザーとして情報更新
		user.Email = email
		user.Username = name
		user.IconURL = picture
		db.Save(&user)
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {

		// B2. UIDで見つからなかった場合 -> Emailで再検索
		emailResult := db.Where("email = ?", email).First(&user)

		if emailResult.Error == nil {
			// C1. Emailで見つかった場合 -> 既存ユーザーにUIDを紐付け（Link Firebase UID）
			user.FirebaseUID = firebaseUID // UIDを登録し直す
			user.Email = email
			user.Username = name
			user.IconURL = picture
			if err := db.Save(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link UID to existing user"})
				return
			}
		} else if errors.Is(emailResult.Error, gorm.ErrRecordNotFound) {
			// C2. UIDもEmailも見つからない場合 -> 完全に新規作成
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
			// C3. Email検索でその他のエラー
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Database error during email check: %v", emailResult.Error)})
			return
		}
	} else {
		// B3. UID検索でその他のエラー
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Database error during UID check: %v", result.Error)})
		return
	}

	// 最終結果を返す
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    user,
	})
}
