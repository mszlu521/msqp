package service

import (
	"core/models/requests"
	"testing"
	"user/pb"
)

func TestLoginUsesSmsCodeBeforeLegacyPassword(t *testing.T) {
	req := &pb.LoginParams{SmsCode: "sms-code", Password: "legacy-password"}
	if got := loginSmsCode(req); got != "sms-code" {
		t.Fatalf("login SMS code = %q, want canonical smsCode", got)
	}
}

func TestLoginKeepsLegacyPasswordCompatibility(t *testing.T) {
	req := &pb.LoginParams{Password: "legacy-code"}
	if got := loginSmsCode(req); got != "legacy-code" {
		t.Fatalf("login SMS code = %q, want legacy password fallback", got)
	}
}

func TestIsMobileLoginAcceptsLegacyMigratedClient(t *testing.T) {
	req := &pb.LoginParams{LoginPlatform: requests.Account, SmsCode: "123456"}
	if !isMobileLogin(req) {
		t.Fatal("SMS-only request from the migrated client should use mobile login")
	}
}

func TestIsMobileLoginDoesNotReclassifyAccountPassword(t *testing.T) {
	req := &pb.LoginParams{LoginPlatform: requests.Account, Password: "password"}
	if isMobileLogin(req) {
		t.Fatal("account/password request should not use mobile login")
	}
}

func TestAccountPasswordHashAndLegacyUpgrade(t *testing.T) {
	hash, err := hashAccountPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if valid, legacy := verifyAccountPassword(hash, "secret"); !valid || legacy {
		t.Fatalf("hashed password verification = %v/%v, want true/false", valid, legacy)
	}
	if valid, _ := verifyAccountPassword(hash, "wrong"); valid {
		t.Fatal("wrong password matched bcrypt hash")
	}
	if valid, legacy := verifyAccountPassword("legacy-secret", "legacy-secret"); !valid || !legacy {
		t.Fatalf("legacy verification = %v/%v, want true/true", valid, legacy)
	}
}
