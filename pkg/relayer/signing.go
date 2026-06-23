package relayer

import (
	"fmt"
	"math/big"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/auth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

var depositWalletBatchTypes = apitypes.Types{
	"EIP712Domain": {
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
	},
	"Call": {
		{Name: "target", Type: "address"},
		{Name: "value", Type: "uint256"},
		{Name: "data", Type: "bytes"},
	},
	"Batch": {
		{Name: "wallet", Type: "address"},
		{Name: "nonce", Type: "uint256"},
		{Name: "deadline", Type: "uint256"},
		{Name: "calls", Type: "Call[]"},
	},
}

func SignDepositWalletBatch(signer auth.Signer, chainID int64, wallet common.Address, nonce string, deadline string, calls []Call) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("relayer: signer is required")
	}
	nonceInt, ok := new(big.Int).SetString(nonce, 10)
	if !ok {
		return nil, fmt.Errorf("relayer: invalid nonce %q", nonce)
	}
	deadlineInt, ok := new(big.Int).SetString(deadline, 10)
	if !ok {
		return nil, fmt.Errorf("relayer: invalid deadline %q", deadline)
	}
	callMessages := make([]any, 0, len(calls))
	for _, call := range calls {
		valueInt, ok := new(big.Int).SetString(call.Value, 10)
		if !ok {
			return nil, fmt.Errorf("relayer: invalid call value %q", call.Value)
		}
		callMessages = append(callMessages, apitypes.TypedDataMessage{
			"target": call.Target.Hex(),
			"value":  (*math.HexOrDecimal256)(valueInt),
			"data":   call.Data,
		})
	}
	domain := &apitypes.TypedDataDomain{
		Name:              "DepositWallet",
		Version:           "1",
		ChainId:           (*math.HexOrDecimal256)(big.NewInt(chainID)),
		VerifyingContract: wallet.Hex(),
	}
	message := apitypes.TypedDataMessage{
		"wallet":   wallet.Hex(),
		"nonce":    (*math.HexOrDecimal256)(nonceInt),
		"deadline": (*math.HexOrDecimal256)(deadlineInt),
		"calls":    callMessages,
	}
	return signer.SignTypedData(domain, depositWalletBatchTypes, message, "Batch")
}
