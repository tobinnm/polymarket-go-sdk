package relayer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/GoPolymarket/polymarket-go-sdk/v2/pkg/auth"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	headerBuilderAPIKey     = "POLY_BUILDER_API_KEY"
	headerBuilderPassphrase = "POLY_BUILDER_PASSPHRASE"
	headerBuilderSignature  = "POLY_BUILDER_SIGNATURE"
	headerBuilderTimestamp  = "POLY_BUILDER_TIMESTAMP"
	headerRelayerAPIKey     = "RELAYER_API_KEY"
	headerRelayerAPIAddress = "RELAYER_API_KEY_ADDRESS"
)

type Client struct {
	baseURL    string
	httpClient Doer
	builder    *BuilderCredentials
	relayer    *RelayerCredentials
	chainID    int64
	nowMillis  func() int64
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = BaseURL
	}
	doer := cfg.HTTPClient
	if doer == nil {
		doer = &http.Client{Timeout: 30 * time.Second}
	}
	chainID := cfg.ChainID
	if chainID == 0 {
		chainID = auth.PolygonChainID
	}
	nowMillis := cfg.NowMillis
	if nowMillis == nil {
		nowMillis = func() int64 { return time.Now().UnixMilli() }
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: doer,
		builder:    cfg.Builder,
		relayer:    cfg.Relayer,
		chainID:    chainID,
		nowMillis:  nowMillis,
	}
}

func (c *Client) ExecuteDepositWalletBatch(ctx context.Context, req ExecuteDepositWalletBatchRequest) (TransactionResponse, error) {
	if req.Signer == nil {
		return TransactionResponse{}, fmt.Errorf("relayer: signer is required")
	}
	if req.DepositWallet.Hex() == "0x0000000000000000000000000000000000000000" {
		return TransactionResponse{}, fmt.Errorf("relayer: deposit wallet is required")
	}
	if req.Deadline == "" {
		return TransactionResponse{}, fmt.Errorf("relayer: deadline is required")
	}
	if len(req.Calls) == 0 {
		return TransactionResponse{}, fmt.Errorf("relayer: calls are required")
	}
	nonce := req.Nonce
	if nonce == "" {
		got, err := c.GetNonce(ctx, req.Signer.Address().Hex(), TransactionTypeWallet)
		if err != nil {
			return TransactionResponse{}, err
		}
		nonce = got.Nonce
	}
	if nonce == "" {
		return TransactionResponse{}, fmt.Errorf("relayer: empty wallet nonce")
	}

	signature, err := SignDepositWalletBatch(req.Signer, c.chainID, req.DepositWallet, nonce, req.Deadline, req.Calls)
	if err != nil {
		return TransactionResponse{}, err
	}
	payload := SubmitRequest{
		Type:      TransactionTypeWallet,
		From:      req.Signer.Address(),
		To:        DepositWalletFactoryPolygon,
		Nonce:     nonce,
		Signature: hexutil.Encode(signature),
		DepositWalletParams: DepositWalletParams{
			DepositWallet: req.DepositWallet,
			Deadline:      req.Deadline,
			Calls:         req.Calls,
		},
	}
	var resp TransactionResponse
	if err := c.post(ctx, "/submit", payload, &resp); err != nil {
		return TransactionResponse{}, err
	}
	return resp, nil
}

func (c *Client) GetNonce(ctx context.Context, address string, txType string) (nonceResponse, error) {
	q := url.Values{}
	q.Set("address", address)
	q.Set("type", txType)
	var resp nonceResponse
	if err := c.do(ctx, http.MethodGet, "/nonce", q, nil, &resp); err != nil {
		return nonceResponse{}, err
	}
	return resp, nil
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("relayer: marshal request: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, nil, raw, out)
}

func (c *Client) do(ctx context.Context, method string, path string, query url.Values, body []byte, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("relayer: parse url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	pathForSignature := u.Path
	if u.RawQuery != "" {
		pathForSignature += "?" + u.RawQuery
	}
	var reader io.Reader
	bodyString := ""
	if body != nil {
		bodyString = string(body)
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return fmt.Errorf("relayer: build request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	headers, err := c.authHeaders(method, pathForSignature, bodyString)
	if err != nil {
		return err
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("relayer: request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("relayer: read response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("relayer: status %d body=%s", response.StatusCode, string(responseBody))
	}
	if out != nil {
		if err := json.Unmarshal(responseBody, out); err != nil {
			return fmt.Errorf("relayer: decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) authHeaders(method string, path string, body string) (http.Header, error) {
	if c.relayer != nil {
		if c.relayer.Key == "" || c.relayer.Address.Hex() == "0x0000000000000000000000000000000000000000" {
			return nil, fmt.Errorf("relayer: relayer credentials are incomplete")
		}
		headers := http.Header{}
		headers.Set(headerRelayerAPIKey, c.relayer.Key)
		headers.Set(headerRelayerAPIAddress, c.relayer.Address.Hex())
		return headers, nil
	}
	if c.builder == nil {
		return nil, fmt.Errorf("relayer: relayer or builder credentials are required")
	}
	if c.builder.Key == "" || c.builder.Secret == "" || c.builder.Passphrase == "" {
		return nil, fmt.Errorf("relayer: builder credentials are required")
	}
	ts := c.nowMillis()
	message := fmt.Sprintf("%d%s%s%s", ts, method, path, body)
	sig, err := auth.SignHMAC(c.builder.Secret, message)
	if err != nil {
		return nil, fmt.Errorf("relayer: sign builder headers: %w", err)
	}
	headers := http.Header{}
	headers.Set(headerBuilderAPIKey, c.builder.Key)
	headers.Set(headerBuilderPassphrase, c.builder.Passphrase)
	headers.Set(headerBuilderTimestamp, fmt.Sprintf("%d", ts))
	headers.Set(headerBuilderSignature, sig)
	return headers, nil
}
