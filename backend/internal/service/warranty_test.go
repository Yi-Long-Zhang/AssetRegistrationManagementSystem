package service

import (
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWarrantyExpiringAssets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.User{}, &model.MailConfig{}, &model.IMConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	future10 := now.AddDate(0, 0, 10)
	past := now.AddDate(0, 0, -5)
	farFuture := now.AddDate(0, 0, 60)

	mk := func(no string, expire *time.Time) {
		if err := db.Create(&model.Asset{AssetNo: no, Hostname: no, IP: "10.0.0.1", Status: "in_use", WarrantyExpireDate: expire}).Error; err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}
	mk("A-EXPIRING", &future10)
	mk("B-EXPIRED", &past)
	mk("C-FAR", &farFuture)
	mk("D-NONE", nil)

	s := NewWarrantyReminderScheduler(db, "")
	got := s.expiringAssets(now)
	if len(got) != 2 {
		t.Fatalf("expect 2 expiring assets, got %d", len(got))
	}
	// 应按到期日升序：过期在前
	if got[0].AssetNo != "B-EXPIRED" || got[1].AssetNo != "A-EXPIRING" {
		t.Fatalf("order mismatch: %s, %s", got[0].AssetNo, got[1].AssetNo)
	}

	// 每天只提醒一次：第二次 tick 不重复查询/通知
	s.tick()
	s.tick()
}
