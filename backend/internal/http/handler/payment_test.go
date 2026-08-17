package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPaymentNotifyParamsSupportsQueryAndFormPost(t *testing.T) {
	getReq, err := http.NewRequest(http.MethodGet, "/notify?pid=1001&out_trade_no=P1&money=1.00", nil)
	if err != nil {
		t.Fatal(err)
	}
	getParams, err := paymentNotifyParams(getReq)
	if err != nil || getParams["out_trade_no"] != "P1" || getParams["money"] != "1.00" {
		t.Fatalf("GET params = %#v, err %v", getParams, err)
	}

	form := url.Values{
		"pid":          {"1001"},
		"out_trade_no": {"P2"},
		"trade_no":     {"T2"},
		"money":        {"2.50"},
	}
	postReq, err := http.NewRequest(http.MethodPost, "/notify", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postParams, err := paymentNotifyParams(postReq)
	if err != nil || postParams["out_trade_no"] != "P2" || postParams["trade_no"] != "T2" || postParams["money"] != "2.50" {
		t.Fatalf("POST params = %#v, err %v", postParams, err)
	}
}
