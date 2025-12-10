package database

import (
	"errors"
	"fmt"
	"os"

	"github.com/Kousuke-irie/hackathon-backend/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DBClient はGORMのクライアントを保持する構造体
var DBClient *gorm.DB

// InitDB データベース接続とマイグレーションを実行
func InitDB() error {
	var dsn string

	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	cloudSQLConnName := os.Getenv("CLOUD_SQL_CONNECTION_NAME")

	dsn = fmt.Sprintf("%s:%s@unix(/cloudsql/%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbUser, dbPassword, cloudSQLConnName, dbName)

	var err error
	DBClient, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	// マイグレーション
	err = DBClient.AutoMigrate(
		&models.User{}, &models.Item{}, &models.Transaction{},
		&models.Like{}, &models.Comment{}, &models.Community{}, &models.CommunityPost{},
		&models.Category{}, &models.ProductCondition{}, &models.Review{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	fmt.Println("Database migration completed!")

	if err := SeedData(DBClient); err != nil {
		return fmt.Errorf("failed to seed data: %w", err)
	}

	return nil
}

func SeedData(db *gorm.DB) error {

	// 1. 外部キーチェックを一時的にオフ
	db.Exec("SET FOREIGN_KEY_CHECKS = 0;")

	// 2. TRUNCATE TABLE でデータを全て削除し、AUTO_INCREMENTをリセット
	if err := db.Exec("TRUNCATE TABLE `categories`;").Error; err != nil {
		db.Exec("SET FOREIGN_KEY_CHECKS = 1;")
		return fmt.Errorf("failed to truncate categories: %w", err)
	}

	if err := db.Exec("TRUNCATE TABLE `product_conditions`;").Error; err != nil {
		db.Exec("SET FOREIGN_KEY_CHECKS = 1;")
		return fmt.Errorf("failed to truncate product_conditions: %w", err)
	}

	// 3. 外部キーチェックをオンに戻す
	db.Exec("SET FOREIGN_KEY_CHECKS = 1;")

	// カテゴリの初期データ
	mainCategories := []struct {
		ID       int
		Name     string
		IconName string
	}{
		{ID: 1, Name: "レディース", IconName: "Woman"},
		{ID: 2, Name: "メンズ", IconName: "Man"},
		{ID: 3, Name: "ベビー・キッズ", IconName: "ChildCare"},
		{ID: 4, Name: "インテリア・住まい・小物", IconName: "Home"},
		{ID: 5, Name: "家電・スマホ・カメラ", IconName: "Devices"},
		{ID: 6, Name: "本・音楽・ゲーム", IconName: "Book"},
		{ID: 7, Name: "ホビー・楽器", IconName: "Hobby"},
		{ID: 8, Name: "コスメ・美容", IconName: "Cosmetics"},
		{ID: 9, Name: "スポーツ・レジャー", IconName: "Sports"},
		{ID: 10, Name: "ハンドメイド", IconName: "Handmade"},
		{ID: 11, Name: "自動車・オートバイ", IconName: "Car"},
		{ID: 12, Name: "チケット", IconName: "Ticket"},
		{ID: 13, Name: "アクセサリー", IconName: "Jewelry"},
		{ID: 14, Name: "食品・飲料", IconName: "Food"},
		{ID: 15, Name: "ペット用品", IconName: "Pets"},
		{ID: 16, Name: "その他", IconName: "Other"},
	}

	parentIDs := make(map[int]uint)

	for _, catData := range mainCategories {
		category := models.Category{Name: catData.Name, IconName: catData.IconName, ParentID: nil}
		db.FirstOrCreate(&category, models.Category{Name: catData.Name, ParentID: nil})
		parentIDs[catData.ID] = category.ID
	}

	subCategories := []struct {
		Parent int // Parent ID (1〜16)
		Name   string
	}{
		// 1. レディース (Parent ID 1)
		{Parent: 1, Name: "レディース トップス"},
		{Parent: 1, Name: "レディース ジャケット / アウター"},
		{Parent: 1, Name: "レディース パンツ"},
		{Parent: 1, Name: "スカート"},
		{Parent: 1, Name: "ワンピース"},
		{Parent: 1, Name: "レディース バッグ"},
		{Parent: 1, Name: "レディース アクセサリー"},
		{Parent: 1, Name: "レディース 靴"},
		{Parent: 1, Name: "レディース 小物"},
		{Parent: 1, Name: "レディース ルームウェア"},

		// 2. メンズ (Parent ID 2)
		{Parent: 2, Name: "メンズ トップス"},
		{Parent: 2, Name: "メンズ ジャケット / アウター"},
		{Parent: 2, Name: "メンズ パンツ"},
		{Parent: 2, Name: "メンズ 靴"},
		{Parent: 2, Name: "メンズ バッグ"},
		{Parent: 2, Name: "メンズ アクセサリー"},
		{Parent: 2, Name: "メンズ 小物"},

		// 3. ベビー・キッズ (Parent ID 3)
		{Parent: 3, Name: "ベビー服"},
		{Parent: 3, Name: "キッズ服"},
		{Parent: 3, Name: "おもちゃ"},
		{Parent: 3, Name: "ベビー用品"},
		{Parent: 3, Name: "キッズ用家具"},

		// 4. インテリア・住まい・小物 (Parent ID 4)
		{Parent: 4, Name: "家具"},
		{Parent: 4, Name: "寝具"},
		{Parent: 4, Name: "キッチン/食器"},
		{Parent: 4, Name: "収納家具"},
		{Parent: 4, Name: "照明"},
		{Parent: 4, Name: "カーテン"},
		{Parent: 4, Name: "ラグ/マット"},

		// 5. 家電・スマホ・カメラ (Parent ID 5)
		{Parent: 5, Name: "スマホ本体"},
		{Parent: 5, Name: "スマホアクセサリー"},
		{Parent: 5, Name: "パソコン"},
		{Parent: 5, Name: "PC周辺機器"},
		{Parent: 5, Name: "カメラ"},
		{Parent: 5, Name: "生活家電（冷蔵庫・炊飯器・掃除機など）"},
		{Parent: 5, Name: "テレビ / 映像機器"},

		// 6. 本・音楽・ゲーム (Parent ID 6)
		{Parent: 6, Name: "本・漫画"},
		{Parent: 6, Name: "CD"},
		{Parent: 6, Name: "DVD / Blu-ray"},
		{Parent: 6, Name: "ゲームソフト"},
		{Parent: 6, Name: "ゲーム機本体"},

		// 7. ホビー・楽器 (Parent ID 7)
		{Parent: 7, Name: "フィギュア"},
		{Parent: 7, Name: "プラモデル"},
		{Parent: 7, Name: "トレカ（遊戯王・ポケカなど）"},
		{Parent: 7, Name: "楽器"},
		{Parent: 7, Name: "模型"},
		{Parent: 7, Name: "アニメグッズ"},

		// 8. コスメ・美容 (Parent ID 8)
		{Parent: 8, Name: "スキンケア"},
		{Parent: 8, Name: "メイクアップ"},
		{Parent: 8, Name: "香水"},
		{Parent: 8, Name: "ヘアケア"},
		{Parent: 8, Name: "ボディケア"},

		// 9. スポーツ・レジャー (Parent ID 9)
		{Parent: 9, Name: "スポーツウェア"},
		{Parent: 9, Name: "アウトドア用品（テント・バーナーなど）"},
		{Parent: 9, Name: "ゴルフ用品"},
		{Parent: 9, Name: "自転車関連"},
		{Parent: 9, Name: "トレーニング用品"},

		// 10. ハンドメイド (Parent ID 10)
		{Parent: 10, Name: "アクセサリー"},
		{Parent: 10, Name: "ファッション"},
		{Parent: 10, Name: "雑貨"},
		{Parent: 10, Name: "素材・材料"},

		// 11. 自動車・オートバイ (Parent ID 11)
		{Parent: 11, Name: "車本体"},
		{Parent: 11, Name: "バイク本体"},
		{Parent: 11, Name: "カーパーツ"},
		{Parent: 11, Name: "バイクパーツ"},
		{Parent: 11, Name: "アクセサリー（ヘルメット・カー用品など）"},

		// 12. チケット (Parent ID 12)
		{Parent: 12, Name: "コンサート"},
		{Parent: 12, Name: "スポーツ"},
		{Parent: 12, Name: "舞台 / 演劇"},
		{Parent: 12, Name: "イベント一般"},

		// 13. アクセサリー (Parent ID 13)
		{Parent: 13, Name: "ブレスレット"},
		{Parent: 13, Name: "指輪"},
		{Parent: 13, Name: "イヤリング / ピアス"},
		{Parent: 13, Name: "ネックレス"},
		{Parent: 13, Name: "腕時計"},

		// 14. 食品・飲料 (Parent ID 14)
		{Parent: 14, Name: "スイーツ"},
		{Parent: 14, Name: "缶詰"},
		{Parent: 14, Name: "調味料"},
		{Parent: 14, Name: "飲料"},
		{Parent: 14, Name: "ギフトセット"},

		// 15. ペット用品 (Parent ID 15)
		{Parent: 15, Name: "犬用品"},
		{Parent: 15, Name: "猫用品"},
		{Parent: 15, Name: "鳥・小動物用品"},
		{Parent: 15, Name: "エサ"},
		{Parent: 15, Name: "ペット家具"},

		// 16. その他 (Parent ID 16)
		{Parent: 16, Name: "まとめ売り"},
		{Parent: 16, Name: "ジャンル不明"},
		{Parent: 16, Name: "個人制作物"},
		{Parent: 16, Name: "雑貨"},
	}

	for _, sub := range subCategories {
		parentID := parentIDs[sub.Parent]
		var existingCategory models.Category
		result := db.Where("name = ?", sub.Name).First(&existingCategory)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			newCategory := models.Category{Name: sub.Name, ParentID: &parentID, IconName: "Sub"}
			db.Create(&newCategory)
		} else if existingCategory.ParentID == nil {
			var checkExisting models.Category
			// NameとParentIDが一致するレコードがあるか確認 (重複登録を防ぐ)
			db.Where("name = ? AND parent_id = ?", sub.Name, parentID).First(&checkExisting)

			if checkExisting.ID == 0 {
				// まだ存在しない場合のみ新規作成
				newCategory := models.Category{Name: sub.Name, ParentID: &parentID, IconName: "Sub"}
				db.Create(&newCategory)
			}
		}
	}

	// 商品状態の初期データ
	conditions := []models.ProductCondition{
		{Name: "新品、未使用", Rank: 1},
		{Name: "未使用に近い", Rank: 2},
		{Name: "目立った傷や汚れなし", Rank: 3},
		{Name: "やや傷や汚れあり", Rank: 4},
		{Name: "傷や汚れあり", Rank: 5},
		{Name: "全体的に状態が悪い", Rank: 6},
	}
	for _, cond := range conditions {
		db.FirstOrCreate(&cond, models.ProductCondition{Name: cond.Name})
	}
	return nil
}
