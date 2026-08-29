package clobtypes

import (
	"encoding/json"
	"testing"
)

func TestTradeDecodesMakerOrdersBreakdown(t *testing.T) {
	payload := `{
		"id": "trade-1",
		"price": "0.45",
		"size": "30",
		"side": "BUY",
		"status": "CONFIRMED",
		"market": "0xcond",
		"asset_id": "123",
		"match_time": "1756000000",
		"maker_orders": [
			{"order_id": "0xa", "owner": "key-1", "matched_amount": "20", "price": "0.45", "asset_id": "123", "outcome": "Yes", "side": "SELL", "fee_rate_bps": "0"},
			{"order_id": "0xb", "owner": "key-2", "matched_amount": "10", "price": "0.45", "asset_id": "123", "outcome": "Yes", "side": "SELL", "fee_rate_bps": "0"}
		]
	}`
	var tr Trade
	if err := json.Unmarshal([]byte(payload), &tr); err != nil {
		t.Fatal(err)
	}
	if len(tr.MakerOrders) != 2 {
		t.Fatalf("maker orders: %+v", tr.MakerOrders)
	}
	if tr.MakerOrders[0].OrderID != "0xa" || tr.MakerOrders[0].MatchedAmount != "20" {
		t.Errorf("first maker order: %+v", tr.MakerOrders[0])
	}
}
