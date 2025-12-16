package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Kousuke-irie/hackathon-backend/database"
	"github.com/Kousuke-irie/hackathon-backend/models"
	"github.com/gin-gonic/gin"
)

type PostReviewRequest struct {
	RaterID uint64 `json:"rater_id" binding:"required"`
	Rating  int    `json:"rating" binding:"required"` // 評価点 (例: 1-5)
	Comment string `json:"comment"`
	Role    string `json:"role" binding:"required"` // 評価者の役割 ('BUYER' or 'SELLER')
}

// UpdateTransactionStatusHandler ステータスを更新（発送、受け取りなど）
func UpdateTransactionStatusHandler(c *gin.Context) {
	txIDStr := c.Param("tx_id")
	txID, err := strconv.ParseUint(txIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	var req struct {
		NewStatus string `json:"new_status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	// 💡 権限チェック: ここでは省略しますが、出品者または購入者のみが実行できるべきです。

	// ステータスを更新
	if err := database.DBClient.Model(&models.Transaction{}).
		Where("id = ?", txID).
		Update("status", req.NewStatus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	if req.NewStatus == "SHIPPED" {
		var tx models.Transaction
		database.DBClient.Preload("Item").First(&tx, txID)

		noti := models.Notification{
			UserID:    tx.BuyerID,
			Type:      "SHIPPED",
			Content:   fmt.Sprintf("商品「%s」が発送されました。到着までお待ちください", tx.Item.Title),
			RelatedID: tx.ItemID,
		}
		database.DBClient.Create(&noti)
		BroadcastNotification(tx.BuyerID, noti)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status updated", "new_status": req.NewStatus})
}

// PostReviewHandler 評価を投稿し、取引ステータスを更新（受け取り完了）
func PostReviewHandler(c *gin.Context) {
	txIDStr := c.Param("tx_id")
	txID, err := strconv.ParseUint(txIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	var req PostReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	db := database.DBClient

	newReview := models.Review{
		TransactionID: txID,
		RaterID:       req.RaterID,
		Rating:        req.Rating,
		Comment:       req.Comment,
		Role:          req.Role,
	}

	if err := db.Create(&newReview).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post review"})
		return
	}

	// 3. 取引ステータスを 'RECEIVED' または 'COMPLETED' に更新 (ここでは 'RECEIVED' にする)
	// 💡 注意: 相手側も評価を完了すると 'COMPLETED' に遷移させるのが理想的ですが、
	//          今回は購入者の評価をもって一旦 'RECEIVED' とします。
	if err := db.Model(&models.Transaction{}).
		Where("id = ?", txID).
		Update("Status", "RECEIVED").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update transaction status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Review posted and status updated"})
}

// CancelTransactionHandler 取引をキャンセル
func CancelTransactionHandler(c *gin.Context) {
	txIDStr := c.Param("tx_id")
	txID, err := strconv.ParseUint(txIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	db := database.DBClient
	var tx models.Transaction

	// 1. 取引の現在のステータスと存在を確認
	if err := db.First(&tx, txID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// 2. 💡 重要なチェック: 既に発送済み（SHIPPED）でないかを確認
	if tx.Status == "SHIPPED" || tx.Status == "COMPLETED" || tx.Status == "CANCELED" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cancellation is not allowed for shipped or completed transactions."})
		return
	}

	// 3. ステータスを CANCELED に更新
	if err := db.Model(&tx).Update("Status", "CANCELED").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel transaction"})
		return
	}

	// 4. 💡 関連する商品（Item）のステータスもON_SALEに戻す（在庫復活）
	// ※ 厳密には取引キャンセルと同時に在庫を戻すべきですが、ここでは Item ID が必要
	if err := db.Model(&models.Item{}).Where("id = ?", tx.ItemID).Update("Status", "ON_SALE").Error; err != nil {
		// 在庫の復元に失敗しても、取引自体はキャンセル済みとして続行
		fmt.Printf("Warning: Failed to restore item status for item ID %d", tx.ItemID)
	}

	database.DBClient.Preload("Item").First(&tx, txID)

	// 評価された側（この場合は出品者）に通知
	noti := models.Notification{
		UserID:    tx.SellerID,
		Type:      "RECEIVED",
		Content:   fmt.Sprintf("「%s」の受取評価が完了しました。取引完了です！", tx.Item.Title),
		RelatedID: tx.ItemID,
	}
	database.DBClient.Create(&noti)
	BroadcastNotification(tx.SellerID, noti)

	c.JSON(http.StatusOK, gin.H{"message": "Transaction canceled successfully"})
}
