package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRegisterParamsBindsJSON(t *testing.T) {
	var got RegisterParams
	err := json.Unmarshal([]byte(`{"account":"13800138000","password":"123456","loginPlatform":1,"smsCode":"654321"}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Account != "13800138000" || got.Password != "123456" || got.LoginPlatform != 1 || got.SmsCode != "654321" {
		t.Fatalf("unexpected JSON binding result: %+v", got)
	}
}

func TestAuthParamsBindForm(t *testing.T) {
	form := url.Values{
		"phoneNumber": {"13800138000"},
		"token":       {"token-value"},
	}
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var sms SMSCodeParams
	if err := binding.FormPost.Bind(req, &sms); err != nil {
		t.Fatal(err)
	}
	if sms.PhoneNumber != "13800138000" {
		t.Fatalf("unexpected SMS form binding result: %+v", sms)
	}

	var recon ReconnectionParams
	if err := binding.FormPost.Bind(req, &recon); err != nil {
		t.Fatal(err)
	}
	if recon.Token != "token-value" {
		t.Fatalf("unexpected reconnection form binding result: %+v", recon)
	}
}
