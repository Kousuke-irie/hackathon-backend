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
		items.POST("/upload-url", handlers.GetGcsUploadUrlHandler)
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
		comm.PUT("/:id", handlers.UpdateCommunityHandler)
		comm.DELETE("/:id", handlers.DeleteCommunityHandler)
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
		// 1. ヘッダーから ID を取得
		userIDStr := c.GetHeader("X-User-ID")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-ID header is required"})
			return
		}

		// 2. 文字列を uint64 に変換。エラーがあれば即座に 400 を返す
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid User ID format in header"})
			return
		}

		var notifications []models.Notification

		// 3. データベース検索
		// 💡 修正ポイント: クエリを分割して確実に取得し、Order の指定を文字列で明示する
		db := database.DBClient
		if err := db.Where("user_id = ?", userID).Order("id DESC").Find(&notifications).Error; err != nil {
			// ここで 500 エラーが発生する場合、詳細をレスポンスに含めて原因を特定する
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Database query failed",
				"details": err.Error(),
			})
			return
		}

		// 4. 結果が null の場合は明示的に空配列にする (フロントエンドの .map でのエラー防止)
		if notifications == nil {
			notifications = []models.Notification{}
		}

		c.JSON(http.StatusOK, gin.H{"notifications": notifications})
	})

}
