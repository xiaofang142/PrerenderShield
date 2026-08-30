package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"prerender-shield/internal/redis"
)

// goTOTP 与 Google Authenticator 同算法生成 6 位码（供端到端断言）
func goTOTP(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return ""
	}
	counter := uint64(t.Unix() / 30)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	o := sum[len(sum)-1] & 0xF
	code := (binary.BigEndian.Uint32(sum[o:o+4]) & 0x7FFFFFFF) % 1000000
	return fmt.Sprintf("%06d", code)
}

func newTest2FA(t *testing.T) *TwoFactorAuth {
	t.Helper()
	client, err := redis.NewClient("localhost:6379", "", 0)
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	return NewTwoFactorAuth(client, "PrerenderShield-Test")
}

func TestTwoFactorAuth_FullLifecycle(t *testing.T) {
	tfa := newTest2FA(t)
	uid := "2fa-e2e-user"

	// 清理残留
	_ = tfa.Disable2FA(uid, "000000")

	secret, qr, err := tfa.Enable2FA(uid)
	if err != nil {
		t.Fatalf("Enable2FA: %v", err)
	}
	if secret == "" || qr == "" {
		t.Fatal("secret/qr must be non-empty")
	}

	// 未启用 2FA 时 Verify 静默放行（设计：由登录流程先查 Is2FAEnabled 再决定二步验证）
	if err := tfa.Verify2FA(uid, "any"); err != nil {
		t.Fatalf("verify on non-enabled user must pass through, got %v", err)
	}

	// 正确码激活
	if err := tfa.Confirm2FA(uid, goTOTP(secret, time.Now())); err != nil {
		t.Fatalf("Confirm2FA with valid TOTP: %v", err)
	}
	// 错误码拒绝
	if err := tfa.Confirm2FA(uid, "000000"); err == nil {
		t.Fatal("invalid code must be rejected")
	}

	// 激活后 Verify 通过
	if err := tfa.Verify2FA(uid, goTOTP(secret, time.Now())); err != nil {
		t.Fatalf("Verify2FA after confirm: %v", err)
	}
	if on, err := tfa.Is2FAEnabled(uid); err != nil || !on {
		t.Fatalf("Is2FAEnabled must be true after confirm (on=%v err=%v)", on, err)
	}

	// 备份码生成与验证
	codes, err := tfa.GenerateBackupCodes(uid)
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(codes) == 0 {
		t.Fatal("backup codes empty")
	}
	if err := tfa.VerifyBackupCode(uid, codes[0]); err != nil {
		t.Fatalf("VerifyBackupCode: %v", err)
	}
	// 同一备份码不可重复使用
	if err := tfa.VerifyBackupCode(uid, codes[0]); err == nil {
		t.Fatal("backup code must be single-use")
	}

	// 禁用需有效码
	if err := tfa.Disable2FA(uid, "000000"); err == nil {
		t.Fatal("disable with wrong code must fail")
	}
	if err := tfa.Disable2FA(uid, goTOTP(secret, time.Now())); err != nil {
		t.Fatalf("Disable2FA with valid code: %v", err)
	}
	if on, err := tfa.Is2FAEnabled(uid); err != nil || on {
		t.Fatalf("2FA must be off after disable (on=%v err=%v)", on, err)
	}
}

func TestHashBackupCode(t *testing.T) {
	// 确定性 sha256 hex
	if hashBackupCode("abc") != hashBackupCode("abc") {
		t.Fatal("hash must be deterministic")
	}
	if hashBackupCode("abc") == hashBackupCode("abd") {
		t.Fatal("different codes must hash differently")
	}
	if len(hashBackupCode("x")) != 64 {
		t.Fatal("sha256 hex length")
	}
}
