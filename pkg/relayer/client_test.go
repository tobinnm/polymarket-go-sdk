package relayer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/auth"
	"github.com/ethereum/go-ethereum/common"
)

type captureDoer struct {
	responses map[string]string
	requests  []capturedRequest
}

type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

func (d *captureDoer) Do(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
	}
	d.requests = append(d.requests, capturedRequest{
		Method: req.Method,
		Path:   req.URL.Path,
		Query:  req.URL.RawQuery,
		Header: req.Header.Clone(),
		Body:   body,
	})
	key := req.URL.Path
	if req.URL.RawQuery != "" {
		key += "?" + req.URL.RawQuery
	}
	payload := d.responses[key]
	if payload == "" && req.URL.Path == "/nonce" {
		payload = `{"nonce":"7"}`
	}
	if payload == "" {
		payload = `{}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(payload)),
		Header:     make(http.Header),
	}, nil
}

func TestNegRiskMergeCallCalldataMatchesAdapterSelector(t *testing.T) {
	call, err := NewNegRiskMergePositionsCall(NegRiskMergePositionsCallRequest{
		CollateralToken:    PUSDPolygon,
		ParentCollectionID: common.Hash{},
		ConditionID:        common.HexToHash("0x2ae461a814fa0d75fef7f4007894fcba57e8f92eca71d9d15ac53716b7376d9f"),
		Amount:             big.NewInt(37_647_000),
	})
	if err != nil {
		t.Fatalf("NewNegRiskMergePositionsCall() error = %v", err)
	}
	if call.Target != NegRiskCtfCollateralAdapterPolygon {
		t.Fatalf("target = %s, want %s", call.Target.Hex(), NegRiskCtfCollateralAdapterPolygon.Hex())
	}
	if !strings.HasPrefix(call.Data, "0x9e7212ad") {
		t.Fatalf("merge calldata selector = %s, want 0x9e7212ad", call.Data[:10])
	}
	if !strings.Contains(call.Data, strings.ToLower(strings.TrimPrefix(PUSDPolygon.Hex(), "0x"))) {
		t.Fatalf("merge calldata does not contain pUSD address: %s", call.Data)
	}
}

func TestMergeCallCalldataMatchesConditionalTokensSelector(t *testing.T) {
	call, err := NewMergePositionsCall(MergePositionsCallRequest{
		CollateralToken:    PUSDPolygon,
		ParentCollectionID: common.Hash{},
		ConditionID:        common.HexToHash("0x225f931bb23f04575fe97df6bc53bd104b26625bdd879906a5e12dd27c042d48"),
		Amount:             big.NewInt(60_000_000),
	})
	if err != nil {
		t.Fatalf("NewMergePositionsCall() error = %v", err)
	}
	if call.Target != ConditionalTokensPolygon {
		t.Fatalf("target = %s, want %s", call.Target.Hex(), ConditionalTokensPolygon.Hex())
	}
	if !strings.HasPrefix(call.Data, "0x9e7212ad") {
		t.Fatalf("merge calldata selector = %s, want 0x9e7212ad", call.Data[:10])
	}
	if !strings.Contains(call.Data, strings.ToLower(strings.TrimPrefix(PUSDPolygon.Hex(), "0x"))) {
		t.Fatalf("merge calldata does not contain pUSD address: %s", call.Data)
	}
}

func TestERC20ApproveCallMatchesSelector(t *testing.T) {
	call, err := NewERC20ApproveCall(ERC20ApproveCallRequest{
		Token:   USDCEPolygon,
		Spender: CollateralOnrampPolygon,
		Amount:  big.NewInt(60_000_000),
	})
	if err != nil {
		t.Fatalf("NewERC20ApproveCall() error = %v", err)
	}
	if call.Target != USDCEPolygon {
		t.Fatalf("target = %s, want %s", call.Target.Hex(), USDCEPolygon.Hex())
	}
	if !strings.HasPrefix(call.Data, "0x095ea7b3") {
		t.Fatalf("approve calldata selector = %s, want 0x095ea7b3", call.Data[:10])
	}
	if !strings.Contains(call.Data, strings.ToLower(strings.TrimPrefix(CollateralOnrampPolygon.Hex(), "0x"))) {
		t.Fatalf("approve calldata does not contain onramp address: %s", call.Data)
	}
}

func TestCollateralOnrampWrapCallMatchesSelector(t *testing.T) {
	to := common.HexToAddress("0x9c90cad21cb08320fb224eab032ddae311c017ef")
	call, err := NewCollateralOnrampWrapCall(CollateralOnrampWrapCallRequest{
		Asset:  USDCEPolygon,
		To:     to,
		Amount: big.NewInt(60_000_000),
	})
	if err != nil {
		t.Fatalf("NewCollateralOnrampWrapCall() error = %v", err)
	}
	if call.Target != CollateralOnrampPolygon {
		t.Fatalf("target = %s, want %s", call.Target.Hex(), CollateralOnrampPolygon.Hex())
	}
	if !strings.Contains(call.Data, strings.ToLower(strings.TrimPrefix(USDCEPolygon.Hex(), "0x"))) {
		t.Fatalf("wrap calldata does not contain USDC.e address: %s", call.Data)
	}
	if !strings.Contains(call.Data, strings.ToLower(strings.TrimPrefix(to.Hex(), "0x"))) {
		t.Fatalf("wrap calldata does not contain recipient address: %s", call.Data)
	}
}

func TestExecuteDepositWalletBatchSignsAndSubmitsWalletPayload(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner("0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318", 137)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}
	doer := &captureDoer{responses: map[string]string{
		"/nonce?address=0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1&type=WALLET": `{"nonce":"7"}`,
		"/submit": `{"transactionID":"tx-1","state":"STATE_NEW","transactionHash":""}`,
	}}
	client := NewClient(Config{
		BaseURL:    "https://relayer.example",
		HTTPClient: doer,
		Builder: &BuilderCredentials{
			Key:        "builder-key",
			Secret:     "c2VjcmV0",
			Passphrase: "builder-pass",
		},
		NowMillis: func() int64 { return 1_760_000_000_000 },
	})

	resp, err := client.ExecuteDepositWalletBatch(context.Background(), ExecuteDepositWalletBatchRequest{
		Signer:        signer,
		DepositWallet: common.HexToAddress("0x9c90cad21cb08320fb224eab032ddae311c017ef"),
		Deadline:      "1760000600",
		Calls: []Call{{
			Target: PUSDPolygon,
			Value:  "0",
			Data:   "0x095ea7b30000000000000000000000000000000000000000000000000000000000000000",
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteDepositWalletBatch() error = %v", err)
	}
	if resp.TransactionID != "tx-1" {
		t.Fatalf("TransactionID = %q, want tx-1", resp.TransactionID)
	}
	if len(doer.requests) != 2 {
		t.Fatalf("requests = %d, want nonce and submit", len(doer.requests))
	}
	submit := doer.requests[1]
	if submit.Method != http.MethodPost || submit.Path != "/submit" {
		t.Fatalf("submit request = %s %s", submit.Method, submit.Path)
	}
	if submit.Header.Get("POLY_BUILDER_API_KEY") != "builder-key" {
		t.Fatalf("missing builder api key header")
	}
	if submit.Header.Get("POLY_BUILDER_TIMESTAMP") != "1760000000000" {
		t.Fatalf("builder timestamp = %q", submit.Header.Get("POLY_BUILDER_TIMESTAMP"))
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewBufferString(submit.Body)).Decode(&payload); err != nil {
		t.Fatalf("decode submit body: %v", err)
	}
	if payload["type"] != "WALLET" {
		t.Fatalf("payload type = %v, want WALLET", payload["type"])
	}
	if !strings.EqualFold(fmt.Sprint(payload["from"]), signer.Address().Hex()) {
		t.Fatalf("payload from = %v, want %s", payload["from"], signer.Address().Hex())
	}
	if !strings.EqualFold(fmt.Sprint(payload["to"]), DepositWalletFactoryPolygon.Hex()) {
		t.Fatalf("payload to = %v, want %s", payload["to"], DepositWalletFactoryPolygon.Hex())
	}
	if payload["nonce"] != "7" {
		t.Fatalf("payload nonce = %v, want 7", payload["nonce"])
	}
	if sig, ok := payload["signature"].(string); !ok || !strings.HasPrefix(sig, "0x") || len(sig) != 132 {
		t.Fatalf("payload signature = %v, want 65-byte hex signature", payload["signature"])
	}
}

func TestExecuteDepositWalletBatchCanSubmitWithRelayerCredentials(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner("0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318", 137)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}
	doer := &captureDoer{responses: map[string]string{
		"/nonce?address=0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1&type=WALLET": `{"nonce":"7"}`,
		"/submit": `{"transactionID":"tx-1","state":"STATE_NEW"}`,
	}}
	client := NewClient(Config{
		BaseURL:    "https://relayer.example",
		HTTPClient: doer,
		Relayer: &RelayerCredentials{
			Key:     "relayer-key",
			Address: signer.Address(),
		},
	})

	resp, err := client.ExecuteDepositWalletBatch(context.Background(), ExecuteDepositWalletBatchRequest{
		Signer:        signer,
		DepositWallet: common.HexToAddress("0x9c90cad21cb08320fb224eab032ddae311c017ef"),
		Deadline:      "1760000600",
		Calls: []Call{{
			Target: ConditionalTokensPolygon,
			Value:  "0",
			Data:   "0x9e7212ad",
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteDepositWalletBatch() error = %v", err)
	}
	if resp.TransactionID != "tx-1" {
		t.Fatalf("TransactionID = %q, want tx-1", resp.TransactionID)
	}
	submit := doer.requests[1]
	if submit.Header.Get("RELAYER_API_KEY") != "relayer-key" {
		t.Fatalf("relayer key header = %q, want relayer-key", submit.Header.Get("RELAYER_API_KEY"))
	}
	if submit.Header.Get("RELAYER_API_KEY_ADDRESS") != signer.Address().Hex() {
		t.Fatalf("relayer address header = %q, want %s", submit.Header.Get("RELAYER_API_KEY_ADDRESS"), signer.Address().Hex())
	}
	if submit.Header.Get("POLY_BUILDER_API_KEY") != "" {
		t.Fatalf("unexpected builder api key header")
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewBufferString(submit.Body)).Decode(&payload); err != nil {
		t.Fatalf("decode submit body: %v", err)
	}
	if payload["type"] != "WALLET" {
		t.Fatalf("payload type = %v, want WALLET", payload["type"])
	}
	if sig, ok := payload["signature"].(string); !ok || !strings.HasPrefix(sig, "0x") || len(sig) != 132 {
		t.Fatalf("payload signature = %v, want 65-byte hex signature", payload["signature"])
	}
}

func TestExecuteDepositWalletBatchRejectsMissingAuthCredentials(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner("0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318", 137)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner() error = %v", err)
	}
	doer := &captureDoer{responses: map[string]string{
		"/nonce?address=0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1&type=WALLET": `{"nonce":"7"}`,
	}}
	client := NewClient(Config{
		BaseURL:    "https://relayer.example",
		HTTPClient: doer,
	})

	_, err = client.ExecuteDepositWalletBatch(context.Background(), ExecuteDepositWalletBatchRequest{
		Signer:        signer,
		DepositWallet: common.HexToAddress("0x9c90cad21cb08320fb224eab032ddae311c017ef"),
		Deadline:      "1760000600",
		Calls: []Call{{
			Target: ConditionalTokensPolygon,
			Value:  "0",
			Data:   "0x9e7212ad",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "credentials are required") {
		t.Fatalf("ExecuteDepositWalletBatch() error = %v, want credentials required", err)
	}
}
