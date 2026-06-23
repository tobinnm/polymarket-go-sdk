package relayer

import (
	"math/big"
	"net/http"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/auth"
	"github.com/ethereum/go-ethereum/common"
)

const (
	BaseURL = "https://relayer-v2.polymarket.com"

	TransactionTypeWallet = "WALLET"

	StateNew       = "STATE_NEW"
	StateExecuted  = "STATE_EXECUTED"
	StateMined     = "STATE_MINED"
	StateInvalid   = "STATE_INVALID"
	StateConfirmed = "STATE_CONFIRMED"
	StateFailed    = "STATE_FAILED"
)

var (
	DepositWalletFactoryPolygon        = common.HexToAddress("0x00000000000Fb5C9ADea0298D729A0CB3823Cc07")
	PUSDPolygon                        = common.HexToAddress("0xC011a7E12a19f7B1f670d46F03B03f3342E82DFB")
	USDCEPolygon                       = common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	CollateralOnrampPolygon            = common.HexToAddress("0x93070a847efEf7F70739046A929D47a521F5B8ee")
	ConditionalTokensPolygon           = common.HexToAddress("0x4D97DCd97eC945f40cF65F87097ACe5EA0476045")
	NegRiskCtfCollateralAdapterPolygon = common.HexToAddress("0xadA2005600Dec949baf300f4C6120000bDB6eAab")
	BinaryPartition                    = []*big.Int{big.NewInt(1), big.NewInt(2)}
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type BuilderCredentials struct {
	Key        string
	Secret     string
	Passphrase string
}

type RelayerCredentials struct {
	Key     string
	Address common.Address
}

type Config struct {
	BaseURL    string
	HTTPClient Doer
	Builder    *BuilderCredentials
	Relayer    *RelayerCredentials
	ChainID    int64
	NowMillis  func() int64
}

type Call struct {
	Target common.Address `json:"target"`
	Value  string         `json:"value"`
	Data   string         `json:"data"`
}

type nonceResponse struct {
	Nonce string `json:"nonce"`
}

type DepositWalletParams struct {
	DepositWallet common.Address `json:"depositWallet"`
	Deadline      string         `json:"deadline"`
	Calls         []Call         `json:"calls"`
}

type SubmitRequest struct {
	Type                string              `json:"type"`
	From                common.Address      `json:"from"`
	To                  common.Address      `json:"to"`
	Nonce               string              `json:"nonce"`
	Signature           string              `json:"signature"`
	DepositWalletParams DepositWalletParams `json:"depositWalletParams"`
}

type TransactionResponse struct {
	TransactionID   string `json:"transactionID"`
	State           string `json:"state"`
	Hash            string `json:"hash"`
	TransactionHash string `json:"transactionHash"`
}

type ExecuteDepositWalletBatchRequest struct {
	Signer        auth.Signer
	DepositWallet common.Address
	Deadline      string
	Nonce         string
	Calls         []Call
}

type MergePositionsCallRequest struct {
	CollateralToken    common.Address
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	Partition          []*big.Int
	Amount             *big.Int
}

type NegRiskMergePositionsCallRequest struct {
	CollateralToken    common.Address
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	Partition          []*big.Int
	Amount             *big.Int
}

type ERC20ApproveCallRequest struct {
	Token   common.Address
	Spender common.Address
	Amount  *big.Int
}

type CollateralOnrampWrapCallRequest struct {
	Asset  common.Address
	To     common.Address
	Amount *big.Int
}
