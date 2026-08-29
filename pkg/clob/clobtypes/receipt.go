package clobtypes

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// PostOrderReceipt models the POST /order and POST /orders placement
// response. It is distinct from OrderResponse (the GET order query shape):
// placement responses carry success/errorMsg/makingAmount/takingAmount and
// settlement evidence, which callers need for order lifecycle classification.
type PostOrderReceipt struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"errorMsg,omitempty"`
	OrderID  string `json:"orderID,omitempty"`
	Status   string `json:"status,omitempty"`
	// MakingAmount and TakingAmount are decimal strings; the upstream API
	// returns an empty string for zero.
	MakingAmount string `json:"makingAmount,omitempty"`
	TakingAmount string `json:"takingAmount,omitempty"`
	// TransactionsHashes lists settlement transaction hashes on a best-effort
	// basis when the order matched.
	TransactionsHashes []string `json:"transactionsHashes,omitempty"`
	// TradeIDs lists the trades created when the order matched.
	TradeIDs []string `json:"tradeIds,omitempty"`
}

// PostOrdersReceipts is the batch placement response.
type PostOrdersReceipts []PostOrderReceipt

// UnmarshalJSON tolerates the upstream API's field aliases
// ("transactionHashes"/"transactionsHashes", "tradeIds"/"tradeIDs",
// "orderID"/"orderId") and string-or-number amounts.
func (r *PostOrderReceipt) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return err
	}
	next := *r

	if value, ok := raw["success"]; ok {
		if err := json.Unmarshal(value, &next.Success); err != nil {
			return fmt.Errorf("success: %w", err)
		}
	}
	stringFields := []struct {
		keys []string
		dest *string
	}{
		{[]string{"errorMsg"}, &next.ErrorMsg},
		{[]string{"orderID", "orderId"}, &next.OrderID},
		{[]string{"status"}, &next.Status},
		{[]string{"makingAmount"}, &next.MakingAmount},
		{[]string{"takingAmount"}, &next.TakingAmount},
	}
	for _, f := range stringFields {
		for _, key := range f.keys {
			if value, ok := raw[key]; ok {
				if err := unmarshalOrderResponseStringLike(value, f.dest); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
				break
			}
		}
	}
	listFields := []struct {
		keys []string
		dest *[]string
	}{
		{[]string{"transactionsHashes", "transactionHashes"}, &next.TransactionsHashes},
		{[]string{"tradeIds", "tradeIDs"}, &next.TradeIDs},
	}
	for _, f := range listFields {
		for _, key := range f.keys {
			value, ok := raw[key]
			if !ok {
				continue
			}
			valueTrimmed := bytes.TrimSpace(value)
			if len(valueTrimmed) == 0 || bytes.Equal(valueTrimmed, []byte("null")) {
				break
			}
			if err := json.Unmarshal(value, f.dest); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			break
		}
	}
	*r = next
	return nil
}
