package service

import (
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLicenseExpiringLicenses(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SoftwareLicense{}, &model.Asset{}, &model.User{}, &model.Ticket{}, &model.TicketRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	future10 := now.AddDate(0, 0, 10)
	past := now.AddDate(0, 0, -5)
	farFuture := now.AddDate(0, 0, 60)

	if err := db.Create(&model.User{Username: "admin", Role: model.RoleAdmin, Status: "active"}).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	mk := func(name string, expire *time.Time) {
		if err := db.Create(&model.SoftwareLicense{Name: name, Type: model.LicenseTypeCommercial, ExpireDate: expire}).Error; err != nil {
			t.Fatalf("create license: %v", err)
		}
	}
	mk("A-EXPIRING", &future10)
	mk("B-EXPIRED", &past)
	mk("C-FAR", &farFuture)
	mk("D-NONE", nil)

	s := NewLicenseReminderScheduler(db, "")
	got := s.expiringLicenses(now)
	if len(got) != 2 {
		t.Fatalf("expect 2 expiring licenses, got %d", len(got))
	}
	// 应按到期日升序：过期在前
	if got[0].Name != "B-EXPIRED" || got[1].Name != "A-EXPIRING" {
		t.Fatalf("order mismatch: %s, %s", got[0].Name, got[1].Name)
	}

	// 每天只提醒一次：第二次 tick 不重复查询/通知
	s.tick()
	s.tick()
}

func TestSoftwareLicenseServiceEncryption(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SoftwareLicense{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewSoftwareLicenseService(db, "test-encryption-key")
	lic := &model.SoftwareLicense{Name: "Windows Server", Vendor: "Microsoft", Type: model.LicenseTypeCommercial, TotalSeats: 100, UsedSeats: 20}
	if err := svc.Create(lic, "XXXXX-YYYYY-ZZZZZ"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !lic.Encrypted {
		t.Fatalf("expect encrypted flag set")
	}

	// 列表接口不应暴露明文
	var listed []model.SoftwareLicense
	if err := db.Find(&listed).Error; err != nil {
		t.Fatalf("list: %v", err)
	}
	if listed[0].LicenseKey == "XXXXX-YYYYY-ZZZZZ" {
		t.Fatalf("license key leaked in plaintext query")
	}

	// 解密往返
	_, plain, err := svc.Reveal(lic.ID)
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if plain != "XXXXX-YYYYY-ZZZZZ" {
		t.Fatalf("reveal mismatch: %q", plain)
	}

	// 更新元信息不丢密文；重填密钥后解密为新值
	updated := &model.SoftwareLicense{Name: "Windows Server 2025", Vendor: "Microsoft", Type: model.LicenseTypeCommercial, TotalSeats: 100, UsedSeats: 30}
	if err := svc.Update(lic.ID, updated, "NEW-KEY"); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, plain, err = svc.Reveal(lic.ID)
	if err != nil {
		t.Fatalf("reveal after update: %v", err)
	}
	if plain != "NEW-KEY" {
		t.Fatalf("reveal after update mismatch: %q", plain)
	}
}

func TestSoftwareLicenseImportRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SoftwareLicense{}, &model.Asset{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.Asset{AssetNo: "A-1", Hostname: "srv-01", IP: "10.0.0.1", Status: model.AssetStatusInUse}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	svc := NewSoftwareLicenseService(db, "test-encryption-key")
	records := [][]string{
		{"全资产详细清单（表头前的说明行应被跳过）"},
		{"软件名", "厂商", "类型", "许可证密钥", "授权数量", "已用数量", "到期日", "购买日期", "关联资产", "备注"},
		{"Windows Server", "Microsoft", "商业授权", "KEY-1", "100", "20", "2027-01-01", "2026-01-01", "srv-01", "备注A"},
		{"Linux Server", "Red Hat", "subscription", "KEY-2", "50", "5", "", "", "", ""},
		{"Windows Server", "Microsoft", "commercial", "KEY-3", "100", "30", "2027-06-01", "", "10.0.0.1", "更新行"},
	}
	created, updated, err := svc.ImportRows(records)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if created != 2 || updated != 1 {
		t.Fatalf("expect created=2 updated=1, got created=%d updated=%d", created, updated)
	}
	var count int64
	if err := db.Model(&model.SoftwareLicense{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expect 2 licenses after import, got %d", count)
	}
	// 新建行密钥已加密，可解密回原文；关联资产已解析
	var win model.SoftwareLicense
	if err := db.Where("name = ?", "Windows Server").First(&win).Error; err != nil {
		t.Fatalf("find windows license: %v", err)
	}
	if win.LicenseKey == "KEY-3" {
		t.Fatalf("license key stored in plaintext")
	}
	if _, plain, err := svc.Reveal(win.ID); err != nil || plain != "KEY-3" {
		t.Fatalf("reveal updated key: err=%v plain=%q", err, plain)
	}
	if win.UsedSeats != 30 || win.AssetID == nil {
		t.Fatalf("updated row not applied: seats=%d assetId=%v", win.UsedSeats, win.AssetID)
	}
	var linux model.SoftwareLicense
	if err := db.Where("name = ?", "Linux Server").First(&linux).Error; err != nil {
		t.Fatalf("find linux license: %v", err)
	}
	if linux.Type != model.LicenseTypeSubscription {
		t.Fatalf("type normalization failed: %q", linux.Type)
	}
	if _, plain, err := svc.Reveal(linux.ID); err != nil || plain != "KEY-2" {
		t.Fatalf("reveal created key: err=%v plain=%q", err, plain)
	}

	// 新建行密钥必填：整批回滚
	bad := [][]string{
		{"软件名", "厂商", "许可证密钥"},
		{"New App", "Some Vendor", ""},
	}
	if _, _, err := svc.ImportRows(bad); err == nil {
		t.Fatalf("expect error when new row lacks license key")
	}
	// 软件名必填
	bad2 := [][]string{
		{"软件名", "许可证密钥"},
		{"", "KEY-X"},
	}
	if _, _, err := svc.ImportRows(bad2); err == nil {
		t.Fatalf("expect error when name is empty")
	}
	// 关联资产不存在：报错
	bad3 := [][]string{
		{"软件名", "厂商", "许可证密钥", "关联资产"},
		{"App X", "Vendor", "KEY-Y", "no-such-host"},
	}
	if _, _, err := svc.ImportRows(bad3); err == nil {
		t.Fatalf("expect error when referenced asset missing")
	}
}

func TestLicenseRenewalTickets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SoftwareLicense{}, &model.User{}, &model.Ticket{}, &model.TicketRecord{}, &model.TicketAsset{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin := model.User{Username: "admin", Name: "管理员", Role: model.RoleAdmin, Status: "active"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	expiring := time.Now().AddDate(0, 0, 10)
	lic := model.SoftwareLicense{Name: "Windows Server", Vendor: "Microsoft", Type: model.LicenseTypeCommercial, TotalSeats: 100, ExpireDate: &expiring}
	if err := db.Create(&lic).Error; err != nil {
		t.Fatalf("create license: %v", err)
	}

	s := NewLicenseReminderScheduler(db, "")
	s.createRenewalTickets([]model.SoftwareLicense{lic})
	countTickets := func() int64 {
		var n int64
		if err := db.Model(&model.Ticket{}).Count(&n).Error; err != nil {
			t.Fatalf("count tickets: %v", err)
		}
		return n
	}
	if n := countTickets(); n != 1 {
		t.Fatalf("expect 1 renewal ticket, got %d", n)
	}
	var ticket model.Ticket
	if err := db.First(&ticket).Error; err != nil {
		t.Fatalf("load ticket: %v", err)
	}
	if ticket.Type != model.TicketTypeLicenseRenew || ticket.LicenseID == nil || *ticket.LicenseID != lic.ID {
		t.Fatalf("renewal ticket not linked: type=%s licenseId=%v", ticket.Type, ticket.LicenseID)
	}
	if ticket.Status != model.TicketStatusDraft {
		t.Fatalf("renewal ticket should be draft, got %s", ticket.Status)
	}
	// 未关闭的续费工单存在时重复执行不新增
	s.createRenewalTickets([]model.SoftwareLicense{lic})
	if n := countTickets(); n != 1 {
		t.Fatalf("expect still 1 ticket after dedup, got %d", n)
	}
	// 工单关闭后允许再次生成
	if err := db.Model(&ticket).Update("status", model.TicketStatusClosed).Error; err != nil {
		t.Fatalf("close ticket: %v", err)
	}
	s.createRenewalTickets([]model.SoftwareLicense{lic})
	if n := countTickets(); n != 2 {
		t.Fatalf("expect 2 tickets after closing first, got %d", n)
	}
}
