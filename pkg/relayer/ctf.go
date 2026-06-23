package relayer

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var mergePositionsABI abi.ABI

func init() {
	const mergePositionsJSON = `[{"inputs":[{"internalType":"address","name":"collateralToken","type":"address"},{"internalType":"bytes32","name":"parentCollectionId","type":"bytes32"},{"internalType":"bytes32","name":"conditionId","type":"bytes32"},{"internalType":"uint256[]","name":"partition","type":"uint256[]"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"mergePositions","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
	parsed, err := abi.JSON(strings.NewReader(mergePositionsJSON))
	if err != nil {
		panic(fmt.Sprintf("invalid mergePositions abi: %v", err))
	}
	mergePositionsABI = parsed
}

func NewMergePositionsCall(req MergePositionsCallRequest) (Call, error) {
	return newMergePositionsCall(ConditionalTokensPolygon, req.CollateralToken, req.ParentCollectionID, req.ConditionID, req.Partition, req.Amount)
}

func NewNegRiskMergePositionsCall(req NegRiskMergePositionsCallRequest) (Call, error) {
	return newMergePositionsCall(NegRiskCtfCollateralAdapterPolygon, req.CollateralToken, req.ParentCollectionID, req.ConditionID, req.Partition, req.Amount)
}

func newMergePositionsCall(target common.Address, collateralToken common.Address, parentCollectionID common.Hash, conditionID common.Hash, partition []*big.Int, amount *big.Int) (Call, error) {
	if collateralToken == (common.Address{}) {
		collateralToken = PUSDPolygon
	}
	if len(partition) == 0 {
		partition = BinaryPartition
	}
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, fmt.Errorf("relayer: positive merge amount is required")
	}
	data, err := mergePositionsABI.Pack(
		"mergePositions",
		collateralToken,
		parentCollectionID,
		conditionID,
		partition,
		amount,
	)
	if err != nil {
		return Call{}, fmt.Errorf("relayer: pack mergePositions: %w", err)
	}
	return Call{
		Target: target,
		Value:  "0",
		Data:   hexutil.Encode(data),
	}, nil
}

func SharesToRaw6(shares string) (*big.Int, error) {
	value, ok := new(big.Rat).SetString(shares)
	if !ok {
		return nil, fmt.Errorf("relayer: invalid decimal %q", shares)
	}
	value.Mul(value, big.NewRat(1_000_000, 1))
	if !value.IsInt() {
		return nil, fmt.Errorf("relayer: amount %q has more than 6 decimal places", shares)
	}
	if value.Sign() <= 0 {
		return nil, fmt.Errorf("relayer: amount must be positive")
	}
	return value.Num(), nil
}
