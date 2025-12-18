package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/paymentintent"
)

// CreatePaymentIntentHandler 支払い情報の作成
func CreatePaymentIntentHandler(c *gin.Context) {
	// どの商品を買うか受け取る
	var req struct {
		ItemID uint64 `json:"item_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 商品情報をDBから取得（価格を確認するため）
	var item models.Item
	if err := database.DBClient.First(&item, req.ItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// 売り切れチェック
	if item.Status == "SOLD" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This item is already sold out"})
		return
	}

	// Stripeの設定
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	// 支払いインテント作成 (JPYで決済)
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(item.Price)),
		Currency: stripe.String(string(stripe.CurrencyJPY)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	// メタデータに商品IDを入れておく（管理画面で見やすいように）
	params.AddMetadata("item_id", strconv.FormatUint(item.ID, 10))

	pi, err := paymentintent.New(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment intent"})
		return
	}

	// クライアントシークレットを返す
	c.JSON(http.StatusOK, gin.H{
		"clientSecret": pi.ClientSecret,
	})
}

func CompletePurchaseAndCreateTransactionHandler(c *gin.Context) {
	// クライアント（フロントエンド）から、購入者IDと商品IDを受け取る
	var req struct {
		ItemID  uint64 `json:"item_id" binding:"required"`
		BuyerID uint64 `json:"buyer_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: BuyerID and ItemID are required"})
		return
	}

	db := database.DBClient
	var item models.Item

	// 1. 商品情報を取得
	if err := db.First(&item, req.ItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	tx := db.Begin() // トランザクション開始

	// 1. 商品を SOLD に更新 (ON_SALE のものだけを対象にして二重購入防止)
	result := tx.Model(&models.Item{}).
		Where("id = ? AND status = ?", req.ItemID, "ON_SALE").
		Update("status", "SOLD")

	if result.RowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "商品が既に売り切れているか、存在しません"})
		return
	}

	// 2. 取引(Transaction)レコードを作成
	newTx := models.Transaction{
		ItemID:   req.ItemID,
		BuyerID:  req.BuyerID,
		SellerID: item.SellerID,
		Status:   "PURCHASED", // 取引開始
	}
	if err := tx.Create(&newTx).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取引の作成に失敗しました"})
		return
	}

	tx.Commit()

	// 出品者への通知
	noti := models.Notification{
		UserID:    item.SellerID,
		Type:      "SOLD",
		Content:   fmt.Sprintf("祝！「%s」が購入されました。発送準備をお願いします", item.Title),
		RelatedID: item.ID,
	}
	database.DBClient.Create(&noti)
	BroadcastNotification(item.SellerID, noti)

	// 5. 成功レスポンスを返す
	c.JSON(http.StatusOK, gin.H{
		"message":        "Purchase completed and transaction created successfully",
		"transaction_id": newTx.ID,
	})
}
