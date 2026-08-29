package clob

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/auth"
	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/clob/clobtypes"
	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/types"
)

// Well-known anvil/hardhat test key #0. Never a production secret.
const orderHashTestKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func orderHashTestOrder(sigType int) *clobtypes.Order {
	token, _ := new(big.Int).SetString("11015470973684177829729219287262166995141465048508201953575582100565462316088", 10)
	return &clobtypes.Order{
		Salt:          types.U256{Int: big.NewInt(479249096354)},
		TokenID:       types.U256{Int: token},
		MakerAmount:   decimal.NewFromInt(9_000_000),
		TakerAmount:   decimal.NewFromInt(20_000_000),
		Side:          "BUY",
		SignatureType: &sigType,
		Timestamp:     1_756_000_000_000,
	}
}

// The digest OrderHash computes must be exactly what the EOA signing path
// signs: recovering the signature over our digest must yield the signer.
func TestOrderHashMatchesEOASignature(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner(orderHashTestKey, 137)
	if err != nil {
		t.Fatal(err)
	}
	order := orderHashTestOrder(int(auth.SignatureEOA))
	order.Maker = signer.Address()
	order.Signer = signer.Address()

	signed, err := SignOrderWithOptions(signer, &auth.APIKey{Key: "test"}, order, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := OrderHash(&signed.Order, false, 137)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := hexutil.Decode(hash)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := hexutil.Decode(signed.Signature)
	if err != nil || len(sig) != 65 {
		t.Fatalf("signature decode: %v (len=%d)", err, len(sig))
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	pub, err := crypto.SigToPub(digest, sig)
	if err != nil {
		t.Fatal(err)
	}
	if got := crypto.PubkeyToAddress(*pub); got != signer.Address() {
		t.Errorf("recovered %s from OrderHash digest, want %s", got, signer.Address())
	}
}

// Signing with an explicit neg-risk domain must verify against the neg-risk
// order hash and not against the standard one.
func TestSignOrderWithOptionsNegRiskDomain(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner(orderHashTestKey, 137)
	if err != nil {
		t.Fatal(err)
	}
	order := orderHashTestOrder(int(auth.SignatureEOA))
	order.Maker = signer.Address()
	order.Signer = signer.Address()

	signed, err := SignOrderWithOptions(signer, &auth.APIKey{Key: "test"}, order, &SignOrderOptions{NegRisk: true})
	if err != nil {
		t.Fatal(err)
	}
	sig, err := hexutil.Decode(signed.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	recover := func(negRisk bool) string {
		hash, err := OrderHash(&signed.Order, negRisk, 137)
		if err != nil {
			t.Fatal(err)
		}
		digest, _ := hexutil.Decode(hash)
		pub, err := crypto.SigToPub(digest, sig)
		if err != nil {
			t.Fatal(err)
		}
		return crypto.PubkeyToAddress(*pub).Hex()
	}
	if got := recover(true); got != signer.Address().Hex() {
		t.Errorf("neg-risk digest recovered %s, want signer", got)
	}
	if got := recover(false); got == signer.Address().Hex() {
		t.Error("standard-domain digest must not verify a neg-risk signature")
	}
}

func TestOrderHashDomainIsolation(t *testing.T) {
	order := orderHashTestOrder(int(auth.SignatureEOA))
	normal, err := OrderHash(order, false, 137)
	if err != nil {
		t.Fatal(err)
	}
	negRisk, err := OrderHash(order, true, 137)
	if err != nil {
		t.Fatal(err)
	}
	if normal == negRisk {
		t.Error("neg-risk and standard domains must produce different order hashes")
	}
}

// The Poly1271 envelope is innerSig(65) || domainSeparator(32) ||
// contentsHash(32) || orderTypeString || len(orderTypeString) as uint16. The
// embedded components must equal ExchangeDomainSeparator and OrderStructHash.
func TestPoly1271EnvelopeEmbedsOrderHashComponents(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner(orderHashTestKey, 137)
	if err != nil {
		t.Fatal(err)
	}
	order := orderHashTestOrder(int(auth.SignaturePoly1271))
	order.Maker = signer.Address()
	order.Signer = signer.Address()

	signed, err := SignOrderWithOptions(signer, &auth.APIKey{Key: "test"}, order, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hexutil.Decode(signed.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 65+32+32+2 {
		t.Fatalf("envelope too short: %d", len(raw))
	}
	typeLen := binary.BigEndian.Uint16(raw[len(raw)-2:])
	if int(typeLen) != len(poly1271OrderType) {
		t.Fatalf("embedded type length %d, want %d", typeLen, len(poly1271OrderType))
	}
	if got := string(raw[len(raw)-2-int(typeLen) : len(raw)-2]); got != poly1271OrderType {
		t.Errorf("embedded order type string diverged:\n%s", got)
	}
	if got := raw[65:97]; string(got) != string(ExchangeDomainSeparator(false, 137)) {
		t.Error("embedded domain separator disagrees with ExchangeDomainSeparator")
	}
	wantContents, err := OrderStructHash(&signed.Order)
	if err != nil {
		t.Fatal(err)
	}
	if got := raw[97:129]; string(got) != string(wantContents) {
		t.Error("embedded contents hash disagrees with OrderStructHash")
	}
}
