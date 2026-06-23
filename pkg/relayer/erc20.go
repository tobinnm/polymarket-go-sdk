package relayer

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var erc20ABI abi.ABI

func init() {
	const erc20JSON = `[{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`
	parsed, err := abi.JSON(strings.NewReader(erc20JSON))
	if err != nil {
		panic(fmt.Sprintf("invalid erc20 abi: %v", err))
	}
	erc20ABI = parsed
}

func NewERC20ApproveCall(req ERC20ApproveCallRequest) (Call, error) {
	if req.Token == (common.Address{}) {
		return Call{}, fmt.Errorf("relayer: token is required")
	}
	if req.Spender == (common.Address{}) {
		return Call{}, fmt.Errorf("relayer: spender is required")
	}
	if req.Amount == nil || req.Amount.Sign() < 0 {
		return Call{}, fmt.Errorf("relayer: non-negative approve amount is required")
	}
	data, err := erc20ABI.Pack("approve", req.Spender, req.Amount)
	if err != nil {
		return Call{}, fmt.Errorf("relayer: pack approve: %w", err)
	}
	return Call{
		Target: req.Token,
		Value:  "0",
		Data:   hexutil.Encode(data),
	}, nil
}

func ERC20RevokeCall(token common.Address, spender common.Address) (Call, error) {
	return NewERC20ApproveCall(ERC20ApproveCallRequest{
		Token:   token,
		Spender: spender,
		Amount:  big.NewInt(0),
	})
}
