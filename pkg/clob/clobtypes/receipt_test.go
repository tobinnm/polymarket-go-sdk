package clobtypes

import (
	"encoding/json"
	"testing"
)

func TestPostOrderReceiptUnmarshalSuccess(t *testing.T) {
	payload := `{
		"success": true,
		"errorMsg": "",
		"orderID": "0x52d34a5b8dcb258e1c1d7a19e1c6ff2ee03be2f22ded0f8e97b49b0e05e9c1f3",
		"status": "live",
		"makingAmount": "9",
		"takingAmount": "20",
		"transactionsHashes": null,
		"tradeIds": ["t-1", "t-2"]
	}`
	var r PostOrderReceipt
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Success || r.Status != "live" || r.OrderID == "" {
		t.Errorf("unexpected receipt: %+v", r)
	}
	if r.MakingAmount != "9" || r.TakingAmount != "20" {
		t.Errorf("amounts: %+v", r)
	}
	if r.TransactionsHashes != nil {
		t.Errorf("null hashes must stay nil: %+v", r.TransactionsHashes)
	}
	if len(r.TradeIDs) != 2 {
		t.Errorf("trade ids: %+v", r.TradeIDs)
	}
}

// Production quirk shape: success=true with an UNKNOWN status and a rejection
// message. The receipt must surface these fields verbatim so callers can
// normalize it to a terminal rejection.
func TestPostOrderReceiptUnmarshalFakeSuccess(t *testing.T) {
	payload := `{
		"success": true,
		"errorMsg": "not enough balance / allowance",
		"orderID": "",
		"status": "UNKNOWN",
		"makingAmount": "",
		"takingAmount": ""
	}`
	var r PostOrderReceipt
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Success || r.Status != "UNKNOWN" || r.ErrorMsg == "" {
		t.Errorf("quirk fields must survive decoding: %+v", r)
	}
	if r.MakingAmount != "" || r.TakingAmount != "" {
		t.Errorf("empty amounts must stay empty strings: %+v", r)
	}
}

func TestPostOrderReceiptUnmarshalAliases(t *testing.T) {
	payload := `{
		"success": true,
		"orderId": "0xabc",
		"transactionHashes": ["0x1"],
		"tradeIDs": ["t-1"],
		"makingAmount": 9.5
	}`
	var r PostOrderReceipt
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		t.Fatal(err)
	}
	if r.OrderID != "0xabc" {
		t.Errorf("orderId alias: %+v", r)
	}
	if len(r.TransactionsHashes) != 1 || len(r.TradeIDs) != 1 {
		t.Errorf("list aliases: %+v", r)
	}
	if r.MakingAmount != "9.5" {
		t.Errorf("numeric amount must decode as string: %+v", r)
	}
}

func TestPostOrdersReceiptsUnmarshalBatch(t *testing.T) {
	payload := `[
		{"success": true, "orderID": "0x1", "status": "live"},
		{"success": false, "errorMsg": "order rejected", "status": "UNKNOWN"}
	]`
	var rs PostOrdersReceipts
	if err := json.Unmarshal([]byte(payload), &rs); err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || !rs[0].Success || rs[1].Success {
		t.Errorf("batch: %+v", rs)
	}
}
