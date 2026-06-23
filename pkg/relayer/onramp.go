package relayer

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var collateralOnrampABI abi.ABI

func init() {
	const onrampJSON = `[{"inputs":[{"internalType":"address","name":"_asset","type":"address"},{"internalType":"address","name":"_to","type":"address"},{"internalType":"uint256","name":"_amount","type":"uint256"}],"name":"wrap","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
	parsed, err := abi.JSON(strings.NewReader(onrampJSON))
	if err != nil {
		panic(fmt.Sprintf("invalid collateral onramp abi: %v", err))
	}
	collateralOnrampABI = parsed
}

func NewCollateralOnrampWrapCall(req CollateralOnrampWrapCallRequest) (Call, error) {
	if req.Asset == (common.Address{}) {
		return Call{}, fmt.Errorf("relayer: wrap asset is required")
	}
	if req.To == (common.Address{}) {
		return Call{}, fmt.Errorf("relayer: wrap recipient is required")
	}
	if req.Amount == nil || req.Amount.Sign() <= 0 {
		return Call{}, fmt.Errorf("relayer: positive wrap amount is required")
	}
	data, err := collateralOnrampABI.Pack("wrap", req.Asset, req.To, req.Amount)
	if err != nil {
		return Call{}, fmt.Errorf("relayer: pack wrap: %w", err)
	}
	return Call{
		Target: CollateralOnrampPolygon,
		Value:  "0",
		Data:   hexutil.Encode(data),
	}, nil
}
