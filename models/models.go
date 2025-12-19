package models

import (
	"time"
)

// User ユーザー
type User struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FirebaseUID string    `gorm:"uniqueIndex;not null" json:"firebase_uid"`
	Email       string    `gorm:"uniqueIndex;not null" json:"email"` // 💡 追加
	Username    string    `json:"username"`
	IconURL     string    `json:"icon_url"`
	Bio         string    `json:"bio" gorm:"type:text"` // 💡 自己紹介
	Address     string    `json:"address"`              // 💡 住所
	Birthdate   string    `json:"birthdate"`            // 💡 生年月日
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Item 商品
type Item struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SellerID      uint64    `gorm:"not null;index" json:"seller_id"`
	Title         string    `gorm:"type:varchar(255);not null" json:"title"`
	Description   string    `gorm:"type:text;not null" json:"description"`
	Price         int       `gorm:"not null" json:"price"`
	ImageURL      string    `gorm:"type:text;not null" json:"image_url"`
	Status        string    `gorm:"type:enum('ON_SALE','SOLD','DRAFT');default:'ON_SALE';not null" json:"status"`
	AITags        string    `gorm:"type:json" json:"ai_tags"`               // MySQL 5.7+ JSON型
	CategoryID    uint      `json:"category_id"`                            // カテゴリID (1:トップス, 2:ボトムス など)
	Condition     string    `gorm:"type:varchar(50)" json:"condition"`      // 商品の状態 (新品、中古など)
	ShippingPayer string    `gorm:"type:varchar(50)" json:"shipping_payer"` // 配送負担者 (seller/buyer)
	ShippingFee   int       `json:"shipping_fee"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Relations
	Seller User `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
}

// Transaction 取引
type Transaction struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ItemID          uint64    `gorm:"not null;index" json:"item_id"`
	BuyerID         uint64    `gorm:"not null;index" json:"buyer_id"`
	SellerID        uint64    `gorm:"not null" json:"seller_id"`
	PriceSnapshot   int       `gorm:"not null" json:"price_snapshot"`
	StripePaymentID string    `gorm:"type:varchar(255)" json:"stripe_payment_id"`
	CreatedAt       time.Time `json:"created_at"`
	Status          string    `gorm:"type:enum('PURCHASED','SHIPPED','COMPLETED','CANCELED');default:'PURCHASED';not null" json:"status"`

	// Relations
	Item  Item `gorm:"foreignKey:ItemID" json:"item,omitempty"`
	Buyer User `gorm:"foreignKey:BuyerID" json:"buyer,omitempty"`
}

// Like スワイプ履歴
type Like struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"not null;index" json:"user_id"`
	ItemID    uint64    `gorm:"not null;index" json:"item_id"`
	Reaction  string    `gorm:"type:enum('LIKE','NOPE');not null" json:"reaction"`
	CreatedAt time.Time `json:"created_at"`
}

// Comment コメント
type Comment struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ItemID    uint64    `gorm:"not null;index" json:"item_id"`
	UserID    uint64    `gorm:"not null" json:"user_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Community コミュニティ
type Community struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	ImageURL    string    `gorm:"type:text" json:"image_url"`
	CreatorID   uint64    `gorm:"not null" json:"creator_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// CommunityPost コミュニティ投稿
type CommunityPost struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CommunityID   uint64    `gorm:"not null;index" json:"community_id"`
	UserID        uint64    `gorm:"not null" json:"user_id"`
	Content       string    `gorm:"type:text" json:"content"`
	RelatedItemID *uint64   `gorm:"index" json:"related_item_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`

	// Relations
	User        User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	RelatedItem *Item `gorm:"foreignKey:RelatedItemID" json:"related_item,omitempty"`
}

// Category 商品カテゴリ
type Category struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name     string `gorm:"type:varchar(50);not null" json:"name"`
	ParentID *uint  `json:"parent_id"`
	IconName string `gorm:"type:varchar(50)" json:"icon_name"`
	// GORMリレーション: 子カテゴリをロードできるようにする (今回は使わないが、将来的に有用)
	Children []Category `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

// ProductCondition 商品の状態
type ProductCondition struct {
	ID   uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"type:varchar(50);not null;unique" json:"name"`
	Rank int    `gorm:"column:rank" json:"rank"` // 状態の順序付け用
}

// Review 取引評価テーブル
type Review struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TransactionID uint64    `gorm:"not null;uniqueIndex" json:"transaction_id"`       // 取引IDと1対1
	RaterID       uint64    `gorm:"not null" json:"rater_id"`                         // 評価したユーザーID (Buyer or Seller)
	Rating        int       `gorm:"not null" json:"rating"`                           // 評価点 (1-5など)
	Comment       string    `gorm:"type:text" json:"comment"`                         // 評価コメント
	Role          string    `gorm:"type:enum('BUYER','SELLER');not null" json:"role"` // 評価者の役割
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Relations
	Transaction Transaction `gorm:"foreignKey:TransactionID" json:"-"`
	Rater       User        `gorm:"foreignKey:RaterID" json:"rater"`
}

// Notification 通知
type Notification struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"not null;index" json:"user_id"`
	Type      string    `gorm:"type:varchar(50);not null" json:"type"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	RelatedID uint64    `json:"related_id"`
	IsRead    bool      `gorm:"default:false;not null" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}
