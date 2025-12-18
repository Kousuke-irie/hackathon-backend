package routes

import (
	"net/http"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/handlers"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// 認証
	r.POST("/login", handlers.LoginHandler)
	r.PUT("/users/me", handlers.UpdateUserHandler)

	// 商品
	items := r.Group("/items")
	{
		items.POST("", handlers.CreateItemHandler)
		items.GET("", handlers.GetItemListHandler)
		items.GET("/:id", handlers.GetItemDetailHandler)
		items.PUT("/:id", handlers.UpdateItemHandler)
		items.POST("/analyze", handlers.AnalyzeItemHandler)
		items.GET("/:id/comments", handlers.GetCommentsHandler)
		items.POST("/:id/comments", handlers.PostCommentHandler)
		items.POST("/:id/sold", handlers.CompletePurchaseAndCreateTransactionHandler)
		items.GET("/by-ids", handlers.GetItemsByIdsHandler)
		items.GET("/:id/liked", handlers.CheckItemLikedHandler)
	}

	// 自分の出品
	my := r.Group("/my")
	{
		my.GET("/items", handlers.GetMyItemsHandler)
		my.GET("/likes", handlers.GetLikedItemsHandler)
		my.GET("/drafts", handlers.GetMyDraftsHandler)
		my.GET("/purchases", handlers.GetMyPurchaseHistoryHandler)
		my.GET("/in-progress", handlers.GetMyPurchasesInProgressHandler)
	}

	// スワイプ
	swipe := r.Group("/swipe")
	{
		swipe.GET("/items", handlers.GetSwipeItemsHandler)
		swipe.POST("/action", handlers.RecordSwipeHandler)
	}

	// 決済
	r.POST("/payment/create-payment-intent", handlers.CreatePaymentIntentHandler)

	// コミュニティ
	comm := r.Group("/communities")
	{
		comm.GET("", handlers.GetCommunitiesHandler)
		comm.POST("", handlers.CreateCommunityHandler)
		comm.GET("/:id/posts", handlers.GetCommunityPostsHandler)
		comm.POST("/:id/posts", handlers.PostToCommunityHandler)
	}

	// ▼▼▼ メタデータ関連 API ▼▼▼
	r.GET("/meta/categories", handlers.GetCategoriesHandler)
	r.GET("/meta/conditions", handlers.GetConditionsHandler)
	r.GET("/meta/categories/tree", handlers.GetCategoryTreeHandler)

	// ▼▼▼  取引関連 API ▼▼▼
	tx := r.Group("/transactions")
	{
		tx.PUT("/:tx_id/status", handlers.UpdateTransactionStatusHandler) // ステータス更新
		tx.POST("/:tx_id/review", handlers.PostReviewHandler)             // 評価投稿
		tx.POST("/:tx_id/cancel", handlers.CancelTransactionHandler)
	}

	// WebSocket エンドポイント
	r.GET("/ws/notifications", handlers.WSNotificationHandler)

	// 通知一覧取得 API (NotificationsPage用)
	r.GET("/my/notifications", func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-ID")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header is required"})
			return
		}
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid User ID format"})
			return
		}
		var notifications []models.Notification
		if err := database.DBClient.Where("user_id = ?", userID).Order("created_at desc").Find(&notifications).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		if notifications == nil {
			notifications = []models.Notification{}
		}
		c.JSON(http.StatusOK, gin.H{"notifications": notifications})
	})

}
