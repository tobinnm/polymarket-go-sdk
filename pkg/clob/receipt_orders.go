package clob

import (
	"context"
	"fmt"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/clob/clobtypes"
)

// PostOrderWithReceipt submits a pre-signed order and returns the full
// placement receipt (success/errorMsg/amounts/settlement evidence), which
// PostOrder's OrderResponse shape does not carry.
func (c *clientImpl) PostOrderWithReceipt(ctx context.Context, req *clobtypes.SignedOrder) (clobtypes.PostOrderReceipt, error) {
	var resp clobtypes.PostOrderReceipt
	payload, err := buildOrderPayload(req)
	if err != nil {
		return resp, err
	}
	err = c.httpClient.Post(ctx, "/order", payload, &resp)
	return resp, mapError(err)
}

// PostOrdersWithReceipts submits multiple pre-signed orders in a single batch
// and returns one placement receipt per order.
func (c *clientImpl) PostOrdersWithReceipts(ctx context.Context, req *clobtypes.SignedOrders) (clobtypes.PostOrdersReceipts, error) {
	var resp clobtypes.PostOrdersReceipts
	if req != nil && len(req.Orders) > clobtypes.MaxPostOrdersBatchSize {
		return resp, fmt.Errorf("batch size %d exceeds maximum of %d orders", len(req.Orders), clobtypes.MaxPostOrdersBatchSize)
	}
	payload, err := buildOrdersPayload(req)
	if err != nil {
		return resp, err
	}
	err = c.httpClient.Post(ctx, "/orders", payload, &resp)
	return resp, mapError(err)
}
