package relayer

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func relayerTestClient(doer *captureDoer) *Client {
	return NewClient(Config{
		BaseURL:    "https://relayer.test",
		HTTPClient: doer,
		Relayer: &RelayerCredentials{
			Key:     "k",
			Address: common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
		},
	})
}

func TestSubmitRawPostsExactBytes(t *testing.T) {
	doer := &captureDoer{responses: map[string]string{
		"/submit": `{"transactionID":"tx-1","state":"STATE_NEW"}`,
	}}
	c := relayerTestClient(doer)
	payload := []byte(`{"type":"WALLET","nonce":"7","metadata":"cmd-1"}`)
	resp, err := c.SubmitRaw(context.Background(), payload)
	if err != nil {
		t.Fatalf("SubmitRaw() error = %v", err)
	}
	if resp.TransactionID != "tx-1" || resp.State != "STATE_NEW" {
		t.Fatalf("resp = %+v", resp)
	}
	if got := doer.requests[0].Body; got != string(payload) {
		t.Fatalf("body reserialized: %q", got)
	}
	if doer.requests[0].Header.Get("RELAYER_API_KEY") != "k" {
		t.Fatal("missing relayer auth header")
	}
}

func TestGetTransactionDecodesAliasesAndList(t *testing.T) {
	doer := &captureDoer{responses: map[string]string{
		"/transaction?id=tx-9": `[{"transactionId":"tx-9","state":"STATE_MINED","hash":"0xabc","nonce":7,"metadata":"cmd-9"}]`,
	}}
	c := relayerTestClient(doer)
	txs, err := c.GetTransaction(context.Background(), "tx-9")
	if err != nil {
		t.Fatalf("GetTransaction() error = %v", err)
	}
	if len(txs) != 1 || txs[0].TransactionID != "tx-9" {
		t.Fatalf("txs = %+v", txs)
	}
	if txs[0].TransactionHash != "0xabc" || txs[0].NonceString() != "7" || txs[0].Metadata != "cmd-9" {
		t.Fatalf("decoded = %+v", txs[0])
	}
}

func TestWalletNonceReadsParamsEndpoint(t *testing.T) {
	doer := &captureDoer{responses: map[string]string{
		"/v1/account/transactions/params?address=0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266&type=WALLET": `{"nonce":12}`,
	}}
	c := relayerTestClient(doer)
	nonce, err := c.WalletNonce(context.Background(), "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	if err != nil {
		t.Fatalf("WalletNonce() error = %v", err)
	}
	if nonce != "12" {
		t.Fatalf("nonce = %q", nonce)
	}
}

func TestConvertPositionsCallEncoding(t *testing.T) {
	call, err := NewConvertPositionsCall(ConvertPositionsCallRequest{
		MarketID: common.HexToHash("0x11"),
		IndexSet: big.NewInt(0b101),
		Amount:   big.NewInt(5_000_000),
	})
	if err != nil {
		t.Fatalf("NewConvertPositionsCall() error = %v", err)
	}
	if call.Target != NegRiskCtfAdapterV2Polygon {
		t.Fatalf("target = %s", call.Target.Hex())
	}
	// convertPositions(bytes32,uint256,uint256) selector.
	if !strings.HasPrefix(call.Data, "0xc64748c4") {
		t.Fatalf("selector = %s", call.Data[:10])
	}
	if _, err := NewConvertPositionsCall(ConvertPositionsCallRequest{
		MarketID: common.HexToHash("0x11"), IndexSet: big.NewInt(0), Amount: big.NewInt(1),
	}); err == nil {
		t.Fatal("empty index set must be rejected")
	}
}

func TestRedeemPositionsCallTargetsMarketClassAdapter(t *testing.T) {
	std, err := NewRedeemPositionsCall(RedeemPositionsCallRequest{
		ConditionID: common.HexToHash("0x22"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if std.Target != StandardCtfAdapterV2Polygon {
		t.Fatalf("standard target = %s", std.Target.Hex())
	}
	neg, err := NewRedeemPositionsCall(RedeemPositionsCallRequest{
		NegRisk: true, ConditionID: common.HexToHash("0x22"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if neg.Target != NegRiskCtfAdapterV2Polygon {
		t.Fatalf("negrisk target = %s", neg.Target.Hex())
	}
	// redeemPositions(address,bytes32,bytes32,uint256[]) selector.
	if !strings.HasPrefix(std.Data, "0x01b7037c") {
		t.Fatalf("selector = %s", std.Data[:10])
	}
}

func TestV2MergeTargetsRecoveryAdapters(t *testing.T) {
	req := MergePositionsCallRequest{
		ConditionID: common.HexToHash("0x33"), Amount: big.NewInt(1_000_000),
	}
	std, err := NewV2MergePositionsCall(false, req)
	if err != nil {
		t.Fatal(err)
	}
	if std.Target != StandardCtfAdapterV2Polygon {
		t.Fatalf("standard target = %s", std.Target.Hex())
	}
	neg, err := NewV2MergePositionsCall(true, req)
	if err != nil {
		t.Fatal(err)
	}
	if neg.Target != NegRiskCtfAdapterV2Polygon {
		t.Fatalf("negrisk target = %s", neg.Target.Hex())
	}
}
