package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// V2 recovery adapters route merge/redeem/convert through the exchange's
// collateral-aware adapters (PUSD wrapping); production capital operations on
// deposit wallets go through these rather than the raw CTF.
var (
	StandardCtfAdapterV2Polygon = common.HexToAddress("0xAdA100Db00Ca00073811820692005400218FcE1f")
	NegRiskCtfAdapterV2Polygon  = common.HexToAddress("0xadA2005600Dec949baf300f4C6120000bDB6eAab")
)

var recoveryAdapterABI abi.ABI

func init() {
	const recoveryAdapterJSON = `[
	 {"inputs":[{"internalType":"address","name":"collateralToken","type":"address"},{"internalType":"bytes32","name":"parentCollectionId","type":"bytes32"},{"internalType":"bytes32","name":"conditionId","type":"bytes32"},{"internalType":"uint256[]","name":"indexSets","type":"uint256[]"}],"name":"redeemPositions","outputs":[],"stateMutability":"nonpayable","type":"function"},
	 {"inputs":[{"internalType":"bytes32","name":"marketId","type":"bytes32"},{"internalType":"uint256","name":"indexSet","type":"uint256"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"convertPositions","outputs":[],"stateMutability":"nonpayable","type":"function"}
	]`
	parsed, err := abi.JSON(strings.NewReader(recoveryAdapterJSON))
	if err != nil {
		panic(fmt.Sprintf("invalid recovery adapter abi: %v", err))
	}
	recoveryAdapterABI = parsed
}

// RedeemPositionsCallRequest targets one resolved condition's payout
// collection. NegRisk selects the V2 adapter for that market class.
type RedeemPositionsCallRequest struct {
	NegRisk            bool
	CollateralToken    common.Address
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	IndexSets          []*big.Int
}

// NewRedeemPositionsCall encodes redeemPositions against the V2 recovery
// adapter for the market class.
func NewRedeemPositionsCall(req RedeemPositionsCallRequest) (Call, error) {
	collateral := req.CollateralToken
	if collateral == (common.Address{}) {
		collateral = PUSDPolygon
	}
	indexSets := req.IndexSets
	if len(indexSets) == 0 {
		indexSets = BinaryPartition
	}
	data, err := recoveryAdapterABI.Pack("redeemPositions",
		collateral, req.ParentCollectionID, req.ConditionID, indexSets)
	if err != nil {
		return Call{}, fmt.Errorf("relayer: pack redeemPositions: %w", err)
	}
	target := StandardCtfAdapterV2Polygon
	if req.NegRisk {
		target = NegRiskCtfAdapterV2Polygon
	}
	return Call{Target: target, Value: "0", Data: hexutil.Encode(data)}, nil
}

// ConvertPositionsCallRequest converts NO positions across a NegRisk event.
// IndexSet is a one-bit-per-question U256 bitmap, not an array index.
type ConvertPositionsCallRequest struct {
	MarketID common.Hash
	IndexSet *big.Int
	Amount   *big.Int
}

// NewConvertPositionsCall encodes NegRiskAdapter convertPositions against the
// NegRisk V2 recovery adapter.
func NewConvertPositionsCall(req ConvertPositionsCallRequest) (Call, error) {
	if req.IndexSet == nil || req.IndexSet.Sign() <= 0 {
		return Call{}, fmt.Errorf("relayer: non-empty index set is required")
	}
	if req.Amount == nil || req.Amount.Sign() <= 0 {
		return Call{}, fmt.Errorf("relayer: positive convert amount is required")
	}
	data, err := recoveryAdapterABI.Pack("convertPositions",
		req.MarketID, req.IndexSet, req.Amount)
	if err != nil {
		return Call{}, fmt.Errorf("relayer: pack convertPositions: %w", err)
	}
	return Call{Target: NegRiskCtfAdapterV2Polygon, Value: "0", Data: hexutil.Encode(data)}, nil
}

// NewV2MergePositionsCall encodes mergePositions against the V2 recovery
// adapter for the market class (the collateral-aware route production merge
// commands use).
func NewV2MergePositionsCall(negRisk bool, req MergePositionsCallRequest) (Call, error) {
	target := StandardCtfAdapterV2Polygon
	if negRisk {
		target = NegRiskCtfAdapterV2Polygon
	}
	return newMergePositionsCall(target, req.CollateralToken, req.ParentCollectionID, req.ConditionID, req.Partition, req.Amount)
}

// Transaction is one relayer transaction record. Field names tolerate the
// endpoint's mixed aliases.
type Transaction struct {
	TransactionID   string          `json:"transactionID"`
	TransactionHash string          `json:"transactionHash,omitempty"`
	Hash            string          `json:"hash,omitempty"`
	State           string          `json:"state"`
	From            string          `json:"from,omitempty"`
	To              string          `json:"to,omitempty"`
	ProxyAddress    string          `json:"proxyAddress,omitempty"`
	Nonce           json.RawMessage `json:"nonce,omitempty"` // string or number
	Signature       string          `json:"signature,omitempty"`
	Type            string          `json:"type,omitempty"`
	Owner           string          `json:"owner,omitempty"`
	Metadata        string          `json:"metadata,omitempty"`
	Error           string          `json:"errorMsg,omitempty"`
}

// UnmarshalJSON tolerates transactionID vs transactionId.
func (t *Transaction) UnmarshalJSON(data []byte) error {
	type wire Transaction
	var w struct {
		wire
		TransactionIDAlt string `json:"transactionId"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*t = Transaction(w.wire)
	if t.TransactionID == "" {
		t.TransactionID = w.TransactionIDAlt
	}
	if t.TransactionHash == "" {
		t.TransactionHash = t.Hash
	}
	return nil
}

// NonceString renders the mixed-encoding nonce as a decimal string.
func (t *Transaction) NonceString() string {
	raw := strings.TrimSpace(string(t.Nonce))
	return strings.Trim(raw, `"`)
}

// SubmitRaw posts exact pre-serialized submit-request bytes. It exists for
// callers with write-ahead persistence: resubmission after a crash must be
// byte-identical to the frozen request, so the client must not re-marshal.
func (c *Client) SubmitRaw(ctx context.Context, payload []byte) (TransactionResponse, error) {
	if len(payload) == 0 {
		return TransactionResponse{}, fmt.Errorf("relayer: empty submit payload")
	}
	var resp TransactionResponse
	if err := c.do(ctx, http.MethodPost, "/submit", nil, payload, &resp); err != nil {
		return TransactionResponse{}, err
	}
	return resp, nil
}

// GetTransaction fetches records for one transaction id. The endpoint is
// list-shaped; callers must filter for the exact id.
func (c *Client) GetTransaction(ctx context.Context, id string) ([]Transaction, error) {
	q := url.Values{}
	q.Set("id", id)
	var out []Transaction
	if err := c.do(ctx, http.MethodGet, "/transaction", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RecentTransactions lists the account's recent relayer transactions (the
// recovery path's ambiguity probe).
func (c *Client) RecentTransactions(ctx context.Context) ([]Transaction, error) {
	var out []Transaction
	if err := c.do(ctx, http.MethodGet, "/transactions", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// WalletNonce reads the account's next deposit-wallet transaction nonce via
// the transactions-params endpoint (the value the batch signature binds).
func (c *Client) WalletNonce(ctx context.Context, address string) (string, error) {
	q := url.Values{}
	q.Set("address", address)
	q.Set("type", TransactionTypeWallet)
	var resp struct {
		Nonce json.RawMessage `json:"nonce"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/account/transactions/params", q, nil, &resp); err != nil {
		return "", err
	}
	nonce := strings.Trim(strings.TrimSpace(string(resp.Nonce)), `"`)
	if nonce == "" {
		return "", fmt.Errorf("relayer: empty wallet nonce")
	}
	return nonce, nil
}
