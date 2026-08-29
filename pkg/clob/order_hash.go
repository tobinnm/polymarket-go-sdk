package clob

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/auth"
	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/clob/clobtypes"
	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/types"
)

// SignOrderOptions carries the signing parameters that the package-level
// SignOrder defaults away: explicit neg-risk domain routing, signature type,
// funder, and salt generation. A nil options value behaves like SignOrder.
type SignOrderOptions struct {
	// NegRisk selects the Neg Risk CTF Exchange V2 verifying contract. It must
	// come from immutable market identity: signing and any later hash
	// recomputation must select the same domain.
	NegRisk bool
	// SignatureType overrides the signature type when the order does not set
	// one.
	SignatureType *auth.SignatureType
	// Funder is the maker address for non-EOA signature types.
	Funder *types.Address
	// SaltGenerator overrides salt generation when the order has no salt.
	SaltGenerator SaltGenerator
}

// SignOrderWithOptions signs an order without posting it. It exists for
// callers that separate signing from submission (for example write-ahead
// persistence of the signed payload) and therefore cannot rely on the
// client's CreateOrder path to resolve the neg-risk domain.
func SignOrderWithOptions(signer auth.Signer, apiKey *auth.APIKey, order *clobtypes.Order, opts *SignOrderOptions) (*clobtypes.SignedOrder, error) {
	if opts == nil {
		opts = &SignOrderOptions{}
	}
	return signOrderWithCreds(signer, apiKey, order, opts.SignatureType, opts.Funder, opts.SaltGenerator, opts.NegRisk)
}

// OrderHash returns the EIP-712 digest of a V2 order under the CTF Exchange
// V2 domain for the given market class and chain, 0x-prefixed. This digest is
// the exchange-side order ID regardless of signature type and is recomputable
// offline from a persisted signed order.
func OrderHash(order *clobtypes.Order, negRisk bool, chainID int64) (string, error) {
	if chainID <= 0 {
		return "", fmt.Errorf("order hash requires a positive chain id, got %d", chainID)
	}
	structHash, err := OrderStructHash(order)
	if err != nil {
		return "", err
	}
	separator := ExchangeDomainSeparator(negRisk, chainID)
	digest := crypto.Keccak256(append(append([]byte{0x19, 0x01}, separator...), structHash...))
	return hexutil.Encode(digest), nil
}

// OrderStructHash returns the EIP-712 struct hash of a V2 order. Poly1271
// signature envelopes embed this value as the contents hash.
func OrderStructHash(order *clobtypes.Order) ([]byte, error) {
	forHash, err := orderForHash(order)
	if err != nil {
		return nil, err
	}
	return poly1271OrderStructHash(forHash)
}

// ExchangeDomainSeparator returns the EIP-712 domain separator of the CTF
// Exchange V2 domain for the given market class and chain.
func ExchangeDomainSeparator(negRisk bool, chainID int64) []byte {
	return poly1271ExchangeDomainSeparator(common.HexToAddress(exchangeV2Address(negRisk)), chainID)
}

// orderForHash mirrors the signing paths exactly: signature type defaults to
// EOA (signOrderWithCreds) and metadata/builder use the same lenient bytes32
// coercion (padBytes32) the signed message uses.
func orderForHash(order *clobtypes.Order) (poly1271OrderForHash, error) {
	if order == nil {
		return poly1271OrderForHash{}, fmt.Errorf("order is required")
	}
	if order.Salt.Int == nil {
		return poly1271OrderForHash{}, fmt.Errorf("order salt is required")
	}
	if order.TokenID.Int == nil {
		return poly1271OrderForHash{}, fmt.Errorf("token_id is required")
	}
	makerAmount := order.MakerAmount.BigInt()
	takerAmount := order.TakerAmount.BigInt()
	if makerAmount == nil || takerAmount == nil {
		return poly1271OrderForHash{}, fmt.Errorf("maker_amount and taker_amount are required")
	}
	side, err := poly1271Side(order.Side)
	if err != nil {
		return poly1271OrderForHash{}, err
	}
	sigType := int(auth.SignatureEOA)
	if order.SignatureType != nil {
		sigType = *order.SignatureType
	}
	return poly1271OrderForHash{
		Salt:          order.Salt.Int,
		Maker:         order.Maker,
		Signer:        order.Signer,
		TokenID:       order.TokenID.Int,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          big.NewInt(int64(side)),
		SignatureType: big.NewInt(int64(sigType)),
		Timestamp:     big.NewInt(order.Timestamp),
		Metadata:      padBytes32(order.Metadata),
		Builder:       padBytes32(order.Builder),
	}, nil
}
