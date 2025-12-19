package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/gemini"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// GetSwipeItemsHandler まだスワイプしていない商品を取得
func GetSwipeItemsHandler(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID header is required"})
		return
	}

	db := database.DBClient

	// 1. ユーザーが過去に「LIKE」した商品のタイトルを取得
	var likedTitles []string
	db.Table("likes").
		Select("items.title").
		Joins("JOIN items ON items.id = likes.item_id").
		Where("likes.user_id = ? AND likes.reaction = ?", userID, "LIKE").
		Order("likes.created_at DESC").
		Limit(10).
		Pluck("title", &likedTitles)

	var items []models.Item
	query := db.Where("status = ?", "ON_SALE").Where("seller_id != ?", userID)

	// すでにスワイプ済みの商品を除外するサブクエリ
	subQuery := db.Table("likes").Select("item_id").Where("user_id = ?", userID)
	query = query.Where("id NOT IN (?)", subQuery)

	// 2. 「LIKE」履歴がある場合、AIで分析して並び替え
	if len(likedTitles) > 0 {
		keywords, err := gemini.AnalyzeUserLikes(c.Request.Context(), likedTitles)
		if err == nil {
			// AIが生成したキーワードで部分一致検索を行い、ヒットするものを優先
			kList := strings.Fields(keywords)
			if len(kList) > 0 {
				var conditions []string
				var values []interface{}
				for _, k := range kList {
					conditions = append(conditions, "title LIKE ? OR description LIKE ?")
					v := "%" + k + "%"
					values = append(values, v, v)
				}
				conditionSql := strings.Join(conditions, " OR ")
				query = query.Clauses(clause.OrderBy{
					Expression: clause.Expr{
						SQL:                fmt.Sprintf("CASE WHEN %s THEN 0 ELSE 1 END", conditionSql),
						Vars:               values,
						WithoutParentheses: true,
					},
				})
			}
		}
	}

	// 3. 最終的な取得（新着順も加味）
	if err := query.Order("created_at DESC").Limit(20).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// RecordSwipeRequest スワイプ記録用のリクエストボディ
type RecordSwipeRequest struct {
	UserID   uint64 `json:"user_id"`
	ItemID   uint64 `json:"item_id"`
	Reaction string `json:"reaction"` // "LIKE" or "NOPE"
}

// RecordSwipeHandler スワイプ結果(Like/Nope)を保存
func RecordSwipeHandler(c *gin.Context) {
	var req RecordSwipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newLike := models.Like{
		UserID:   req.UserID,
		ItemID:   req.ItemID,
		Reaction: req.Reaction,
	}

	if err := database.DBClient.Create(&newLike).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record swipe"})
		return
	}

	if req.Reaction == "LIKE" {
		var item models.Item
		database.DBClient.First(&item, req.ItemID)

		// 相手に通知
		noti := models.Notification{
			UserID:    item.SellerID,
			Type:      "LIKE",
			Content:   fmt.Sprintf("あなたの出品した「%s」にいいね！がつきました", item.Title),
			RelatedID: item.ID,
		}
		database.DBClient.Create(&noti)
		BroadcastNotification(item.SellerID, noti)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Swipe recorded"})
}
