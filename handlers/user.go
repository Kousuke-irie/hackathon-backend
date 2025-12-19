package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UpdateUserRequest ユーザー更新用リクエスト
type UpdateUserRequest struct {
	ID        uint64 `json:"id" binding:"required"`
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	IconURL   string `json:"icon_url"`
	Address   string `json:"address"`   // 追加
	Birthdate string `json:"birthdate"` // 追加
}

// UpdateUserHandler ユーザー情報（プロフィール）を更新
func UpdateUserHandler(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db := database.DBClient
	var user models.User

	// ユーザーの存在確認
	if err := db.First(&user, req.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// 情報の更新
	user.Username = req.Username
	user.Bio = req.Bio
	user.Address = req.Address     // 追加
	user.Birthdate = req.Birthdate // 追加

	if req.IconURL != "" && req.IconURL != user.IconURL {
		user.IconURL = req.IconURL
	}

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated", "user": user})
}

// GetLikedItemsHandler ユーザーがいいねした商品一覧を取得
func GetLikedItemsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID header is required"})
		return
	}

	var items []models.Item
	db := database.DBClient

	// SQL: itemsテーブルとlikesテーブルを結合し、特定のユーザーがLIKEした商品IDをフィルタ
	if err := db.
		Joins("JOIN likes ON likes.item_id = items.id").
		Where("likes.user_id = ? AND likes.reaction = ?", userID, "LIKE").
		Where("items.status = ?", "ON_SALE"). // 販売中のもののみ
		Order("likes.created_at DESC").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch liked items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// CheckItemLikedHandler 特定の商品に対してユーザーがLike済みかチェック
func CheckItemLikedHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{"is_liked": false}) // 未ログインは当然いいねしていない
		return
	}
	itemID := c.Param("id")

	var count int64
	// likes テーブルで、user_idとitem_idが一致し、reactionが'LIKE'のレコードをカウント
	database.DBClient.Model(&models.Like{}).
		Where("user_id = ? AND item_id = ? AND reaction = ?", userID, itemID, "LIKE").
		Count(&count)

	c.JSON(http.StatusOK, gin.H{"is_liked": count > 0})
}

// GetMyPurchaseHistoryHandler 自分の購入履歴を取得
func GetMyPurchaseHistoryHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var transactions []models.Transaction
	db := database.DBClient

	// BuyerIDが自分である取引を取得し、Item情報とSeller情報をPreloadする
	if err := db.Where("buyer_id = ?", userID).
		Preload("Item").
		Preload("Item.Seller"). // 商品の出品者情報も必要なら取得
		Order("created_at DESC").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch purchase history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}

// GetUserByIDHandler ユーザー詳細を取得
func GetUserByIDHandler(c *gin.Context) {
	userID := c.Param("id")
	var user models.User

	// 💡 セキュリティのため、Emailなど非公開にすべき情報は返さないように調整
	if err := database.DBClient.Select("id, username, icon_url, bio, following_count, follower_count, created_at").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// ToggleFollowHandler フォロー/解除を切り替える
func ToggleFollowHandler(c *gin.Context) {
	followerIDStr := c.GetHeader("X-User-ID")
	followerID, _ := strconv.ParseUint(followerIDStr, 10, 64)

	followingIDStr := c.Param("id")
	followingID, _ := strconv.ParseUint(followingIDStr, 10, 64)

	if followerID == followingID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "自分をフォローすることはできません"})
		return
	}

	var follow models.Follow
	db := database.DBClient
	result := db.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&follow)

	if result.Error == nil {
		db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&follow).Error; err != nil {
				return err
			}
			// 💡 カウントを減らす
			tx.Model(&models.User{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("following_count - ?", 1))
			tx.Model(&models.User{}).Where("id = ?", followingID).UpdateColumn("follower_count", gorm.Expr("follower_count - ?", 1))
			return nil
		})
		c.JSON(http.StatusOK, gin.H{"status": "unfollowed"})
	} else {
		// 未フォローならフォロー
		db.Transaction(func(tx *gorm.DB) error {
			newFollow := models.Follow{FollowerID: followerID, FollowingID: followingID}
			if err := tx.Create(&newFollow).Error; err != nil {
				return err
			}
			// 💡 カウントを増やす
			tx.Model(&models.User{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("following_count + ?", 1))
			tx.Model(&models.User{}).Where("id = ?", followingID).UpdateColumn("follower_count", gorm.Expr("follower_count + ?", 1))
			return nil
		})

		// 通知作成
		var follower models.User
		db.First(&follower, followerID)
		noti := models.Notification{
			UserID:    followingID,
			Type:      "SYSTEM",
			Content:   fmt.Sprintf("%sさんにフォローされました", follower.Username),
			RelatedID: followerID,
		}
		db.Create(&noti)
		BroadcastNotification(followingID, noti)

		c.JSON(http.StatusOK, gin.H{"status": "followed"})
	}
}

// GetFollowsHandler フォロー中またはフォロワーの一覧を取得
func GetFollowsHandler(c *gin.Context) {
	userID := c.Param("id")
	mode := c.Query("mode") // "following" or "followers"

	var users []models.User
	db := database.DBClient

	if mode == "following" {
		db.Table("users").
			Joins("JOIN follows ON follows.following_id = users.id").
			Where("follows.follower_id = ?", userID).
			Find(&users)
	} else {
		db.Table("users").
			Joins("JOIN follows ON follows.follower_id = users.id").
			Where("follows.following_id = ?", userID).
			Find(&users)
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// CheckFollowingHandler 特定のユーザーをフォローしているか確認
func CheckFollowingHandler(c *gin.Context) {
	followerID := c.GetHeader("X-User-ID")
	if followerID == "" {
		c.JSON(http.StatusOK, gin.H{"is_following": false})
		return
	}
	followingID := c.Param("id")

	var count int64
	database.DBClient.Model(&models.Follow{}).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Count(&count)

	c.JSON(http.StatusOK, gin.H{"is_following": count > 0})
}

// GetUserReviewsHandler 特定ユーザー宛の評価一覧を取得
func GetUserReviewsHandler(c *gin.Context) {
	userID := c.Param("id")
	var reviews []models.Review
	
	err := database.DBClient.
		Preload("Rater").
		Preload("Transaction.Item").
		Joins("JOIN transactions ON transactions.id = reviews.transaction_id").
		// 出品者としての評価、または購入者としての評価の両方を取得
		// (評価者が自分ではない ＝ 自分が評価された側)
		Where("(transactions.seller_id = ? OR transactions.buyer_id = ?) AND reviews.rater_id != ?", userID, userID, userID).
		Order("reviews.created_at DESC").
		Find(&reviews).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "評価の取得に失敗しました"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reviews": reviews})
}
