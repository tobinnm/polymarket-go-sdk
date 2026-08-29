package cloberrors

import (
	"errors"
	"testing"

	sdkerrors "github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/errors"
	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/types"
)

// Mapped errors must keep the original *types.Error in the wrap chain so
// callers can extract the raw HTTP status and message for classification.
func TestFromTypeErrPreservesOriginalError(t *testing.T) {
	cases := []struct {
		in       *types.Error
		sentinel error
	}{
		{&types.Error{Status: 400, Code: "INSUFFICIENT_FUNDS", Message: "not enough balance"}, sdkerrors.ErrInsufficientFunds},
		{&types.Error{Status: 429, Message: "rate limited"}, sdkerrors.ErrRateLimitExceeded},
		{&types.Error{Status: 503, Message: "trading is disabled"}, sdkerrors.ErrInternalServerError},
		{&types.Error{Status: 401, Message: "bad key"}, sdkerrors.ErrUnauthorized},
	}
	for _, c := range cases {
		mapped := FromTypeErr(c.in)
		if !errors.Is(mapped, c.sentinel) {
			t.Errorf("%v: sentinel lost", c.in)
		}
		var original *types.Error
		if !errors.As(mapped, &original) {
			t.Errorf("%v: original *types.Error lost from chain", c.in)
			continue
		}
		if original.Status != c.in.Status || original.Message != c.in.Message {
			t.Errorf("%v: original fields mutated: %+v", c.in, original)
		}
	}
}
