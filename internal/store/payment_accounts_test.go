package store

import (
	"testing"

	"easysub/internal/config"
)

func TestMigratePaymentAccounts(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{DataDir: t.TempDir(), SQLitePath: "easysub.db"}
	g, err := Open(DBConfig{Path: "easysub.db"})
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	sqlDB, _ := g.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := Migrate(g); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	uid := 7
	acc, note := "alice@example.com", "主力"
	withAcc := PaymentMethod{UserID: &uid, Name: "支付宝", Account: &acc, Note: &note}
	plain := PaymentMethod{UserID: &uid, Name: "PayPal"}
	g.Create(&withAcc)
	g.Create(&plain)
	sub := Subscription{UserID: uid, Name: "Netflix", PaymentMethodID: &withAcc.ID, Currency: "USD", BillingType: "recurring", Cycle: "month", CycleCount: 1}
	g.Create(&sub)

	// 执行两次：第二次不能重复创建账号
	for i := 0; i < 2; i++ {
		if err := Migrate(g); err != nil {
			t.Fatalf("Migrate() #%d: %v", i+2, err)
		}
	}

	var accs []PaymentAccount
	g.Find(&accs)
	if len(accs) != 1 || accs[0].PaymentMethodID != withAcc.ID || accs[0].Account != acc || accs[0].Note != note || accs[0].UserID != uid {
		t.Fatalf("accounts = %+v, want one migrated account", accs)
	}
	var pm PaymentMethod
	g.First(&pm, withAcc.ID)
	if pm.Account != nil || pm.Note != nil {
		t.Fatalf("legacy fields not cleared: %+v", pm)
	}
	var s Subscription
	g.First(&s, sub.ID)
	if s.PaymentAccountID == nil || *s.PaymentAccountID != accs[0].ID {
		t.Fatalf("subscription account = %v, want %d", s.PaymentAccountID, accs[0].ID)
	}
}
