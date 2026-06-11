// Package client provides a Go SDK client for the Treasury API.
//
// The client supports both read operations (GET requests) and
// write operations (signed POST/PUT/DELETE requests using HTTP Message Signatures
// per RFC 9421).
//
// Example usage:
//
//	c, _ := client.NewClient("my-treasury",
//		client.WithBaseURL("http://localhost:8777"),
//	)
//	identity, _ := client.LoadK256Identity("admin", privateKeyHex)
//	c.SetIdentity(identity)
//
//	// Read a resource
//	var account types.Account
//	err := c.Get("accounts/my-account", &account)
//
//	// Create a resource
//	opName, err := c.Create("Account", "my-account", types.Account{...})
package client

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cordialsys/sdk-go/treasury/types"
)

const DefaultBaseURL = "https://treasury.cordialapis.com/"

// Signer is the generic signing interface used for signed write requests.
type Signer interface {
	PublicKey() string
	SigningAlgorithm() SigningAlgorithm
	SignHTTPMessage(signatureBase []byte) ([]byte, error)
}

// Client is the Treasury API client.
type Client struct {
	// BaseURL is the base URL of the Treasury API (e.g. "http://localhost:8777").
	BaseURL string
	// TreasuryID is the treasury identifier used in signed requests.
	TreasuryID string
	// APIKey is sent as Authorization: Bearer <api-key> on every request.
	APIKey string
	// HTTPClient is the HTTP client used for requests. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
	// Identity is the signing identity for write operations. Must be set before calling
	// write methods (Create, Update, Delete, Custom).
	//
	// Deprecated: use Signer or SetSigner for generic signing implementations.
	Identity *Identity
	// Signer is used to sign write requests. SetIdentity also sets this field.
	Signer Signer
}

// ListOptions holds optional parameters for List requests.
type ListOptions struct {
	// Filter is an optional filter string (e.g. "state=active").
	Filter string
	// PageSize is the maximum number of results per page.
	PageSize int
	// PageToken is the token for fetching the next page.
	PageToken string
}

// ListResponse is the response envelope for list operations.
type ListResponse struct {
	// Resources is the list of resources returned as raw JSON.
	Resources []json.RawMessage `json:"resources"`
	// NextPageToken is the token for the next page, if any.
	NextPageToken string `json:"next_page_token,omitempty"`
	// TotalSize is the total number of matching resources.
	TotalSize int `json:"total_size,omitempty"`
}

// APIError represents an error response from the Treasury API.
type APIError struct {
	Err        types.Error
	StatusCode int
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("treasury API error (HTTP %d, code %d, status %s): %s",
		e.StatusCode, e.Err.Code, e.Err.Status, e.Err.Message)
}

type clientConfig struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	signer     Signer
}

// ClientOption configures a Treasury API client.
type ClientOption func(*clientConfig)

// WithBaseURL configures the Treasury API base URL. If omitted, the hosted
// Treasury API is used.
func WithBaseURL(baseURL string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.baseURL = baseURL
	}
}

// WithAPIKey configures the API key used for Authorization on every request.
// Raw API keys are base64-encoded; already-base64 values are preserved.
func WithAPIKey(apiKey string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.apiKey = apiKey
	}
}

// WithHTTPClient configures the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(cfg *clientConfig) {
		cfg.httpClient = httpClient
	}
}

// WithSigner configures the signer used for write requests.
func WithSigner(signer Signer) ClientOption {
	return func(cfg *clientConfig) {
		cfg.signer = signer
	}
}

// NewClient creates a new Treasury API client. treasuryID is required and is
// sent as the Treasury header on every request.
func NewClient(treasuryID string, opts ...ClientOption) (*Client, error) {
	if treasuryID == "" {
		return nil, fmt.Errorf("treasury id is required")
	}
	cfg := clientConfig{
		baseURL: strings.TrimRight(DefaultBaseURL, "/"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	cfg.baseURL = normalizeBaseURL(cfg.baseURL)
	if cfg.baseURL == "" {
		cfg.baseURL = normalizeBaseURL(DefaultBaseURL)
	}
	apiKey := normalizeAPIKey(cfg.apiKey)
	if apiKey == "" && isDefaultBaseURL(cfg.baseURL) {
		return nil, fmt.Errorf("api key is required when using default Treasury API base URL %s", DefaultBaseURL)
	}
	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:    cfg.baseURL,
		TreasuryID: treasuryID,
		APIKey:     apiKey,
		HTTPClient: httpClient,
		Signer:     cfg.signer,
	}, nil
}

// SetIdentity sets the signing identity used for write operations.
func (c *Client) SetIdentity(identity *Identity) {
	c.Identity = identity
	c.Signer = identity
}

// SetSigner sets the signer used for write operations.
func (c *Client) SetSigner(signer Signer) {
	c.Signer = signer
}

// httpClient returns the HTTP client, defaulting to http.DefaultClient.
func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func normalizeBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func isDefaultBaseURL(baseURL string) bool {
	return normalizeBaseURL(baseURL) == normalizeBaseURL(DefaultBaseURL)
}

func normalizeAPIKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ""
	}
	if _, err := base64.StdEncoding.DecodeString(apiKey); err == nil {
		return apiKey
	}
	return base64.StdEncoding.EncodeToString([]byte(apiKey))
}

func (c *Client) applyHeaders(req *http.Request) {
	if c.TreasuryID != "" {
		req.Header.Set("Treasury", c.TreasuryID)
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

// LookupTreasuryID attempts to discover a treasury ID from the API base URL.
// It first tries /v1/treasury, then falls back to listing /v1/treasuries.
func LookupTreasuryID(ctx context.Context, baseURL string, apiKey string) (string, error) {
	baseURL = normalizeBaseURL(baseURL)
	if baseURL == "" {
		baseURL = normalizeBaseURL(DefaultBaseURL)
	}
	apiKey = normalizeAPIKey(apiKey)
	httpClient := &http.Client{Timeout: 30 * time.Second}

	for _, path := range []string{"/v1/treasury", "/v1/treasuries?page_size=1"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			return "", fmt.Errorf("creating treasury lookup request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("looking up treasury id: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("reading treasury lookup response: %w", readErr)
		}
		if resp.StatusCode >= 400 {
			continue
		}
		if id := extractTreasuryID(body); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not discover treasury id from %s", baseURL)
}

func extractTreasuryID(body []byte) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	if name, ok := extractName(obj); ok {
		return treasuryIDFromName(name)
	}
	for key, raw := range obj {
		if key == "next_page_token" || key == "total_size" {
			continue
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
			if name, ok := extractName(items[0]); ok {
				return treasuryIDFromName(name)
			}
		}
	}
	return ""
}

func extractName(obj map[string]json.RawMessage) (string, bool) {
	raw, ok := obj["name"]
	if !ok {
		return "", false
	}
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return "", false
	}
	return name, name != ""
}

func treasuryIDFromName(name string) string {
	if strings.HasPrefix(name, "treasuries/") {
		return strings.TrimPrefix(name, "treasuries/")
	}
	return name
}

// ---------------------------------------------------------------------------
// Resource type to URL path mappings
// ---------------------------------------------------------------------------

// resourceTypeToPathSegment maps a capitalized resource type name to its URL
// path segment (the collection name used in the URL).
var resourceTypeToPathSegment = map[string]string{
	"Account":         "accounts",
	"Address":         "addresses",
	"Asset":           "assets",
	"Credential":      "credentials",
	"Symbol":          "symbols",
	"Transfer":        "transfers",
	"Staking":         "stakings",
	"User":            "users",
	"Key":             "keys",
	"Role":            "roles",
	"AccessRule":      "access-rules",
	"Access-Rule":     "access-rules",
	"TransferRule":    "transfer-rules",
	"Transfer-Rule":   "transfer-rules",
	"CallRule":        "call-rules",
	"Call-Rule":       "call-rules",
	"StakingRule":     "staking-rules",
	"Staking-Rule":    "staking-rules",
	"Operation":       "operations",
	"Transaction":     "transactions",
	"Treasury":        "treasuries",
	"Tag":             "tags",
	"Chain":           "chains",
	"Feature":         "features",
	"Call":            "calls",
	"Signatory":       "signatories",
	"Signer":          "signers",
	"SoftwareUpdate":  "software-updates",
	"Software-Update": "software-updates",
	"Host":            "hosts",
}

// nestedResourceParentSegment maps resource types that are nested under a parent
// to the parent's path segment. For example, Address is nested under chains:
// /v1/chains/{parent}/addresses/{id}
var nestedResourceParentSegment = map[string]string{
	"Address":    "chains",
	"Asset":      "chains",
	"Symbol":     "chains",
	"Call":       "chains",
	"Credential": "users",
}

// resourceTypeToPath converts a ResourceType like "Account" to its URL path segment "accounts".
// Falls back to naive pluralization for unknown types.
func resourceTypeToPath(resourceType string) string {
	if seg, ok := resourceTypeToPathSegment[resourceType]; ok {
		return seg
	}
	// Fallback: lowercase and naive pluralize.
	rt := strings.ToLower(resourceType)
	switch {
	case strings.HasSuffix(rt, "s"):
		return rt + "es"
	case strings.HasSuffix(rt, "y"):
		return rt[:len(rt)-1] + "ies"
	default:
		return rt + "s"
	}
}

// ---------------------------------------------------------------------------
// Read operations
// ---------------------------------------------------------------------------

// Get retrieves a resource by its full resource name (e.g. "accounts/my-account").
// The result is unmarshaled into the provided pointer.
func (c *Client) Get(resourceName string, result interface{}) error {
	raw, err := c.GetJSON(resourceName)
	if err != nil {
		return err
	}
	if result != nil {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}
	return nil
}

// List retrieves a list of resources of the given type.
// resourceType should be the pluralized path segment (e.g. "accounts", "transfers").
func (c *Client) List(resourceType string, opts ListOptions) (*ListResponse, error) {
	u, err := url.Parse(c.BaseURL + "/v1/" + strings.ToLower(resourceType))
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}
	q := u.Query()
	if opts.Filter != "" {
		q.Set("filter", opts.Filter)
	}
	if opts.PageSize > 0 {
		q.Set("page_size", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.PageToken != "" {
		q.Set("page_token", opts.PageToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c.applyHeaders(req)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	// The API returns resources under the collection name key (e.g. {"accounts": [...], "next_page_token": "..."}).
	// Parse the response generically to find the resources array.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshaling list response: %w", err)
	}

	var listResp ListResponse
	if tok, ok := raw["next_page_token"]; ok {
		json.Unmarshal(tok, &listResp.NextPageToken)
	}
	if tot, ok := raw["total_size"]; ok {
		json.Unmarshal(tot, &listResp.TotalSize)
	}

	// Find the resources array: it's the field that is NOT next_page_token or total_size.
	for key, val := range raw {
		if key == "next_page_token" || key == "total_size" {
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(val, &arr); err == nil {
			listResp.Resources = arr
			break
		}
	}
	return &listResp, nil
}

// GetJSON performs a raw GET request and returns the response body as json.RawMessage.
func (c *Client) GetJSON(path string) (json.RawMessage, error) {
	// Ensure path starts with /v1/
	if !strings.HasPrefix(path, "/") {
		path = "/v1/" + path
	}
	u := c.BaseURL + path
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c.applyHeaders(req)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	return json.RawMessage(body), nil
}

// ---------------------------------------------------------------------------
// Write operations (HTTP Message Signatures, RFC 9421)
// ---------------------------------------------------------------------------

// Create creates a new resource. resourceType is the API resource type (e.g. "Account").
// id is the desired resource ID (can be empty to let the server generate one).
// body is the resource data to create.
// Returns the operation name on success.
func (c *Client) Create(resourceType string, id string, body interface{}) (string, error) {
	return c.CreateWithParent(resourceType, id, "", body)
}

// CreateWithParent creates a new nested resource. resourceType is the API resource type
// (e.g. "Address"). id is the desired resource ID (can be empty). parent is the parent
// resource ID (e.g. chain ID "SOL"). body is the resource data to create.
// Returns the operation name on success.
func (c *Client) CreateWithParent(resourceType string, id string, parent string, body interface{}) (string, error) {
	path := buildResourcePath(resourceType, id, parent)
	return c.Execute(http.MethodPost, path, body, "")
}

// Update updates an existing resource. resourceName is the full resource name
// (e.g. "accounts/my-account"). body is the fields to update.
// Returns the operation name on success.
func (c *Client) Update(resourceName string, body interface{}) (string, error) {
	path := "/v1/" + resourceName
	return c.Execute(http.MethodPut, path, body, "")
}

// Delete deletes a resource by its full resource name.
// Returns the operation name on success.
func (c *Client) Delete(resourceName string) (string, error) {
	path := "/v1/" + resourceName
	return c.Execute(http.MethodDelete, path, nil, "")
}

// Custom performs a custom action on a resource.
// resourceName is the full resource name (e.g. "features/my-feature").
// action is the custom action name (e.g. "activate", "disable", "retry", "heartbeat").
// body is optional action-specific data.
// Returns the operation name on success.
func (c *Client) Custom(resourceName string, action string, body interface{}) (string, error) {
	path := "/v1/" + resourceName + "/" + action
	return c.Execute(http.MethodPost, path, body, "")
}

// Approve approves an operation by replaying the original request with an
// approve tag in the HTTP signature. operationID is the operation ID (without
// the "operations/" prefix). Returns the operation name on success.
func (c *Client) Approve(operationID string) (string, error) {
	return c.replayOperation(operationID, "approve")
}

// Cancel cancels an operation by replaying the original request with a
// cancel tag in the HTTP signature. operationID is the operation ID (without
// the "operations/" prefix). Returns the operation name on success.
func (c *Client) Cancel(operationID string) (string, error) {
	return c.replayOperation(operationID, "cancel")
}

// replayOperation fetches an operation's original request and replays it
// with the specified vote tag (approve or cancel).
func (c *Client) replayOperation(operationID string, vote string) (string, error) {
	// 1. Fetch the operation to get its original request details.
	var op types.Operation
	if err := c.Get("operations/"+operationID, &op); err != nil {
		return "", fmt.Errorf("fetching operation for %s: %w", vote, err)
	}

	if op.Request == nil {
		return "", fmt.Errorf("operation %s has no request data", operationID)
	}

	// 2. Reconstruct the HTTP method and path from the operation's request.
	method, path := reconstructRequestPath(op.Request)

	// 3. Reconstruct the body.
	var body interface{}
	if op.Request.Body != nil && *op.Request.Body != "" {
		var bodyMap map[string]interface{}
		if err := json.Unmarshal([]byte(*op.Request.Body), &bodyMap); err == nil {
			body = bodyMap
		} else {
			body = *op.Request.Body
		}
	}

	// 4. Re-execute with the vote tag.
	tag := vote + ":" + operationID
	if _, err := c.Execute(method, path, body, tag); err != nil {
		return "", err
	}
	// Return the original operation name (the one being voted on),
	// matching Rust client behavior which computes the name client-side.
	return "operations/" + operationID, nil
}

// resourceTypeToPlural maps API resource type names to their plural URL path segments.
var resourceTypeToPlural = map[string]string{
	"User":           "users",
	"Role":           "roles",
	"Account":        "accounts",
	"Address":        "addresses",
	"Asset":          "assets",
	"Chain":          "chains",
	"Key":            "keys",
	"Credential":     "credentials",
	"Transfer":       "transfers",
	"Transaction":    "transactions",
	"Feature":        "features",
	"Staking":        "stakings",
	"Operation":      "operations",
	"Signature":      "signatures",
	"Symbol":         "symbols",
	"Tag":            "tags",
	"Signer":         "signers",
	"Signatory":      "signatories",
	"Treasury":       "treasuries",
	"AccessRule":     "access-rules",
	"TransferRule":   "transfer-rules",
	"CallRule":       "call-rules",
	"StakingRule":    "staking-rules",
	"SoftwareUpdate": "software-updates",
	"Host":           "hosts",
	"ClientKey":      "client-keys",
	"AccessPolicy":   "access-policy",
	"TransferPolicy": "transfer-policy",
	"CallPolicy":     "call-policy",
	"StakingPolicy":  "staking-policy",
}

// resourceTypeToParentPlural maps nested resource types to their parent's plural path.
var resourceTypeToParentPlural = map[string]string{
	"Address":    "chains",
	"Asset":      "chains",
	"Symbol":     "chains",
	"Credential": "users",
	"Signatory":  "signers",
}

// reconstructRequestPath maps an operation's Request fields to the HTTP method and URL path.
func reconstructRequestPath(req *types.Request) (method, path string) {
	action := ""
	if req.Action != nil {
		action = string(*req.Action)
	}
	resource := string(req.Resource)

	// Map resource type to plural path segment
	plural, ok := resourceTypeToPlural[resource]
	if !ok {
		plural = strings.ToLower(resource) + "s"
	}

	// Build the base path
	if req.Parent != nil && *req.Parent != "" {
		parentPlural, hasParent := resourceTypeToParentPlural[resource]
		if hasParent {
			path = "/v1/" + parentPlural + "/" + string(*req.Parent) + "/" + plural
		} else {
			// Tag or other nested resource - parent is already qualified
			path = "/v1/" + string(*req.Parent) + "/" + plural
		}
	} else {
		path = "/v1/" + plural
	}

	// Add ID
	if req.Id != nil && *req.Id != "" {
		path += "/" + string(*req.Id)
		if req.Extension != nil && *req.Extension != "" {
			path += "+" + string(*req.Extension)
		}
	}

	// Map action to HTTP method
	switch {
	case action == "create" || action == "generate":
		method = http.MethodPost
	case action == "update":
		method = http.MethodPut
	case action == "delete":
		method = http.MethodDelete
	case strings.HasPrefix(action, "custom/"):
		customAction := strings.TrimPrefix(action, "custom/")
		path += "/" + customAction
		method = http.MethodPost
	default:
		method = http.MethodPost
	}

	return method, path
}

// PollOperation polls an operation until it reaches a terminal state (succeeded or failed).
// operationName can be "operations/ID" or just the path. It returns the final operation state.
func (c *Client) PollOperation(operationName string) (*types.Operation, error) {
	const (
		initialDelay = 500 * time.Millisecond
		maxDelay     = 5 * time.Second
		maxAttempts  = 120
	)

	// Normalize to "operations/ID" format for the Get call.
	if !strings.HasPrefix(operationName, "operations/") {
		operationName = "operations/" + operationName
	}

	delay := initialDelay
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var op types.Operation
		if err := c.Get(operationName, &op); err != nil {
			return nil, fmt.Errorf("polling operation %s: %w", operationName, err)
		}

		if op.State == nil {
			return nil, fmt.Errorf("operation %s has no state", operationName)
		}

		switch *op.State {
		case types.OperationStateSucceeded, types.OperationStateFailed:
			return &op, nil
		case types.OperationStateAuthorizing, types.OperationStateCreatingResource:
			// Still in progress, continue polling.
		default:
			// Unknown state, keep polling.
		}

		time.Sleep(delay)
		// Exponential backoff up to maxDelay.
		delay = delay * 3 / 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return nil, fmt.Errorf("operation %s did not reach terminal state after %d attempts", operationName, maxAttempts)
}

// ---------------------------------------------------------------------------
// Execute: low-level signed request sender
// ---------------------------------------------------------------------------

// Execute sends a signed HTTP request to the Treasury API using HTTP Message
// Signatures (RFC 9421). method is the HTTP method (POST, PUT, DELETE). path is
// the URL path (e.g. "/v1/users/test-go-1"). body is the request body data
// (will be JSON-serialized; nil for empty body). tag is an optional signature
// tag (e.g. "approve:OP_ID" or "cancel:OP_ID"; empty string for none).
// Returns the operation name from the response on success.
func (c *Client) Execute(method, path string, body interface{}, tag string) (string, error) {
	signer := c.Signer
	if signer == nil {
		signer = c.Identity
	}
	if signer == nil {
		return "", fmt.Errorf("signer not set: call SetSigner or SetIdentity before write operations")
	}

	// 1. Serialize the body to JSON.
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("marshaling request body: %w", err)
		}
	}
	if bodyBytes == nil {
		bodyBytes = []byte("{}")
	}

	// 2. Compute Content-Digest header value.
	digest := sha256.Sum256(bodyBytes)
	contentDigest := "sha-256=:" + base64.StdEncoding.EncodeToString(digest[:]) + ":"

	// 3. Parse the URL to extract path and query components.
	parsedURL, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}
	urlPath := parsedURL.Path
	urlQuery := parsedURL.RawQuery
	if urlQuery == "" {
		urlQuery = "?"
	} else {
		urlQuery = "?" + urlQuery
	}

	// 4. Build the signature-params string.
	now := time.Now().Unix()
	nonce := generateNonce()
	sigParams := buildSignatureParams(now, nonce, signer.PublicKey(), string(signer.SigningAlgorithm()), tag)

	// 5. Build the signature base string.
	sigBase := buildSignatureBase(method, urlPath, urlQuery, contentDigest, c.TreasuryID, sigParams)

	// 6. Sign the signature base.
	sigBytes, err := signer.SignHTTPMessage([]byte(sigBase))
	if err != nil {
		return "", fmt.Errorf("signing request: %w", err)
	}
	signatureValue := "iam=:" + base64.StdEncoding.EncodeToString(sigBytes) + ":"

	// 7. Build and send the HTTP request.
	bodyReader := strings.NewReader(string(bodyBytes))

	httpReq, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return "", fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Digest", contentDigest)
	c.applyHeaders(httpReq)
	httpReq.Header.Set("Signature-Input", "iam="+sigParams)
	httpReq.Header.Set("Signature", signatureValue)

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", parseAPIError(resp.StatusCode, respBody)
	}

	// 8. Parse the operation response to extract the operation name.
	return parseOperationName(respBody)
}

// ---------------------------------------------------------------------------
// HTTP Message Signature helpers (RFC 9421)
// ---------------------------------------------------------------------------

// buildSignatureParams constructs the signature-params value for the
// Signature-Input header. Parameters are in alphabetical order matching the
// Rust Treasury implementation: alg, created, keyid, nonce, tag.
// The tag parameter is always present (empty string if no tag).
func buildSignatureParams(created int64, nonce string, keyID, alg, tag string) string {
	return fmt.Sprintf(
		`("@method" "@path" "@query" "content-digest" "treasury");alg="%s";created=%d;keyid="%s";nonce="%s";tag="%s"`,
		alg, created, keyID, nonce, tag,
	)
}

// buildSignatureBase constructs the signature base string that gets signed.
// Special components (@method, @path, @query, @signature-params) are quoted.
// Field components (content-digest, treasury) are NOT quoted per the Rust implementation.
// The signature base ends with a trailing newline matching the Rust signing code.
func buildSignatureBase(method, path, query, contentDigest, treasury, sigParams string) string {
	lines := []string{
		fmt.Sprintf(`"@method": %s`, method),
		fmt.Sprintf(`"@path": %s`, path),
		fmt.Sprintf(`"@query": %s`, query),
		fmt.Sprintf("content-digest: %s", contentDigest),
		fmt.Sprintf("treasury: %s", treasury),
		fmt.Sprintf(`"@signature-params": %s`, sigParams),
	}
	return strings.Join(lines, "\n") + "\n"
}

// ---------------------------------------------------------------------------
// Path building
// ---------------------------------------------------------------------------

// buildResourcePath constructs the URL path for a resource operation.
// resourceType is the capitalized type name (e.g. "Account", "Address").
// id is the resource ID (may be empty for server-generated IDs).
// parent is the parent resource ID (may be empty for top-level resources).
func buildResourcePath(resourceType, id, parent string) string {
	pathSegment := resourceTypeToPath(resourceType)

	// Check if this is a nested resource type.
	if parentSegment, ok := nestedResourceParentSegment[resourceType]; ok && parent != "" {
		// If parent is already qualified (e.g., "users/abc" or "chains/SOL"),
		// use it directly instead of double-prefixing.
		parentPath := parentSegment + "/" + parent
		if strings.HasPrefix(parent, parentSegment+"/") {
			parentPath = parent
		}
		if id != "" {
			return "/v1/" + parentPath + "/" + pathSegment + "/" + id
		}
		return "/v1/" + parentPath + "/" + pathSegment
	}

	// Top-level: /v1/{pathSegment}/{id}
	if id != "" {
		return "/v1/" + pathSegment + "/" + id
	}
	return "/v1/" + pathSegment
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseResourceName splits a resource name like "accounts/my-account" into
// the resource type (capitalized singular form) and the ID.
func parseResourceName(name string) (string, string) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return name, ""
	}
	return pathToResourceType(parts[0]), parts[1]
}

// pathToResourceType converts a URL path segment like "accounts" to the API
// ResourceType like "Account".
func pathToResourceType(pathSegment string) string {
	s := pathSegment
	// Remove common plural suffixes to get singular form.
	switch {
	case strings.HasSuffix(s, "ies"):
		s = s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ses"):
		s = s[:len(s)-2]
	case strings.HasSuffix(s, "s"):
		s = s[:len(s)-1]
	}
	// Capitalize the first letter.
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// generateNonce generates a random nonce string suitable for signature params.
// Matches the Rust Treasury CLI nonce range: 13-digit number.
func generateNonce() string {
	const min int64 = 1_000_000_000_000
	const max int64 = 9_999_999_999_999
	return fmt.Sprintf("%d", min+rand.Int63n(max-min+1))
}

// parseOperationName extracts the operation name from a response body.
// The API returns the operation name as a JSON string: "operations/ID".
func parseOperationName(respBody []byte) (string, error) {
	if len(respBody) == 0 {
		return "", nil
	}

	// The API returns the operation name as a JSON string.
	var opName string
	if err := json.Unmarshal(respBody, &opName); err == nil && opName != "" {
		return opName, nil
	}

	// Fallback: try parsing as an Operation object.
	var op types.Operation
	if err := json.Unmarshal(respBody, &op); err == nil {
		if op.Name != nil {
			return *op.Name, nil
		}
	}

	return string(respBody), nil
}

// parseAPIError attempts to parse an error response body into an APIError.
func parseAPIError(statusCode int, body []byte) error {
	// Try to parse as an error envelope with an "error" field.
	var errorEnvelope struct {
		Error types.Error `json:"error"`
	}
	if err := json.Unmarshal(body, &errorEnvelope); err == nil && errorEnvelope.Error.Message != "" {
		return &APIError{
			Err:        errorEnvelope.Error,
			StatusCode: statusCode,
		}
	}

	// Try to parse as a direct error object.
	var apiErr types.Error
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
		return &APIError{
			Err:        apiErr,
			StatusCode: statusCode,
		}
	}

	// Fallback to raw body.
	return fmt.Errorf("treasury API error (HTTP %d): %s", statusCode, string(body))
}
