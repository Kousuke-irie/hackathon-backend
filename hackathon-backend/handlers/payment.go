package handlers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/paymentintent"
	"github.com/yourname/fleamarket-backend/database"
	"github.com/yourname/fleamarket-backend/models"
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
	params.AddMetadata("item_id", string(rune(item.ID)))

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

// 決済成功後商品ステータスを更新し取引レコードを作成するハンドラ
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

	// 2. 売り切れチェック（念のため）
	if item.Status != "ON_SALE" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item is not available for purchase"})
		return
	}

	// 3. 取引(Transaction)レコードの作成
	newTransaction := models.Transaction{
		ItemID:        req.ItemID,
		SellerID:      item.SellerID,
		BuyerID:       req.BuyerID,
		PriceSnapshot: item.Price, // 取引時の価格を記録
		// 最初のステータスは 'PURCHASED' (購入者が支払いを完了したが、出品者はまだ発送していない状態)
		Status: "PURCHASED",
	}

	if err := db.Create(&newTransaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction record"})
		return
	}

	// 4. 💡 商品(Item)のステータスを 'IN_PROGRESS' に更新
	//    これにより、商品が売約済みとなり、他のユーザーが購入できなくなる。
	//    (取引が完了/キャンセルされるまでこのステータスを維持)
	if err := db.Model(&models.Item{}).Where("id = ?", req.ItemID).Update("Status", "IN_PROGRESS").Error; err != nil {
		// 取引作成は成功しているので、ここでは警告ログを出す
		fmt.Printf("Warning: Failed to update item status to IN_PROGRESS for item ID %d", req.ItemID)
	}

	// 5. 成功レスポンスを返す
	c.JSON(http.StatusOK, gin.H{
		"message":        "Purchase completed and transaction created successfully",
		"transaction_id": newTransaction.ID,
	})
}
