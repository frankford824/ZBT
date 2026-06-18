package externaltool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	platformdb "github.com/frankford824/ZBT/backend/internal/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("external tool resource not found")
	ErrInvalidRequest = errors.New("invalid external tool request")
)

const (
	TransportStreamableHTTP = "streamable_http"

	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusBlocked = "blocked"

	defaultTimeoutMS = 5000
	minTimeoutMS     = 500
	maxTimeoutMS     = 60000
	maxSummaryRunes  = 1000
)

type Store struct {
	pool   *pgxpool.Pool
	client *http.Client
}

type Config struct {
	ID              string         `json:"id"`
	ProviderKey     string         `json:"provider_key"`
	Name            string         `json:"name"`
	Transport       string         `json:"transport"`
	Endpoint        string         `json:"endpoint"`
	Command         string         `json:"command"`
	Enabled         bool           `json:"enabled"`
	AllowedTools    []string       `json:"allowed_tools"`
	TimeoutMS       int            `json:"timeout_ms"`
	MonthlyBudget   float64        `json:"monthly_budget"`
	RedactionPolicy string         `json:"redaction_policy"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type AuditLog struct {
	ID              string         `json:"id"`
	UserID          *string        `json:"user_id"`
	ConfigID        *string        `json:"config_id"`
	ToolProvider    string         `json:"tool_provider"`
	ToolName        string         `json:"tool_name"`
	RequestHash     string         `json:"request_hash"`
	RequestSummary  string         `json:"request_summary"`
	ResponseSummary string         `json:"response_summary"`
	LatencyMS       int            `json:"latency_ms"`
	Status          string         `json:"status"`
	ErrorMessage    string         `json:"error_message"`
	EstimatedCost   float64        `json:"estimated_cost"`
	ResourceType    string         `json:"resource_type"`
	ResourceID      *string        `json:"resource_id"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
}

type UpsertConfigRequest struct {
	Name            string         `json:"name"`
	Transport       string         `json:"transport"`
	Endpoint        string         `json:"endpoint"`
	Command         string         `json:"command"`
	Enabled         *bool          `json:"enabled"`
	AllowedTools    []string       `json:"allowed_tools"`
	TimeoutMS       int            `json:"timeout_ms"`
	MonthlyBudget   float64        `json:"monthly_budget"`
	RedactionPolicy string         `json:"redaction_policy"`
	Metadata        map[string]any `json:"metadata"`
}

type InvokeRequest struct {
	ToolName     string         `json:"tool_name"`
	Arguments    map[string]any `json:"arguments"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
}

type InvokeResult struct {
	ProviderKey string         `json:"provider_key"`
	ToolName    string         `json:"tool_name"`
	Status      string         `json:"status"`
	Result      any            `json:"result"`
	Audit       AuditLog       `json:"audit"`
	Metadata    map[string]any `json:"metadata"`
}

type auditInput struct {
	TenantID        string
	UserID          string
	ConfigID        string
	ToolProvider    string
	ToolName        string
	RequestHash     string
	RequestSummary  string
	ResponseSummary string
	LatencyMS       int
	Status          string
	ErrorMessage    string
	EstimatedCost   float64
	ResourceType    string
	ResourceID      string
	Metadata        map[string]any
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:   pool,
		client: newExternalToolHTTPClient(),
	}
}

func (s *Store) ListConfigs(ctx context.Context, tenantID string) ([]Config, error) {
	configs := []Config{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, provider_key, name, transport, endpoint, command, enabled,
				allowed_tools, timeout_ms, monthly_budget::float8, redaction_policy,
				metadata, created_at, updated_at
			from external_tool_configs
			where tenant_id = $1
			order by provider_key
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanConfig(rows)
			if err != nil {
				return err
			}
			configs = append(configs, item)
		}
		return rows.Err()
	})
	return configs, err
}

func (s *Store) UpsertConfig(ctx context.Context, tenantID, providerKey string, req UpsertConfigRequest) (Config, error) {
	normalized, err := normalizeConfig(providerKey, req)
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		metadataRaw, _ := json.Marshal(normalized.Metadata)
		created, err := scanConfig(tx.QueryRow(ctx, `
			insert into external_tool_configs (
				tenant_id, provider_key, name, transport, endpoint, command, enabled,
				allowed_tools, timeout_ms, monthly_budget, redaction_policy, metadata
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)
			on conflict (tenant_id, provider_key) do update
			set name = excluded.name,
				transport = excluded.transport,
				endpoint = excluded.endpoint,
				command = excluded.command,
				enabled = excluded.enabled,
				allowed_tools = excluded.allowed_tools,
				timeout_ms = excluded.timeout_ms,
				monthly_budget = excluded.monthly_budget,
				redaction_policy = excluded.redaction_policy,
				metadata = excluded.metadata,
				updated_at = now()
			returning id::text, provider_key, name, transport, endpoint, command, enabled,
				allowed_tools, timeout_ms, monthly_budget::float8, redaction_policy,
				metadata, created_at, updated_at
		`, tenantID, normalized.ProviderKey, normalized.Name, normalized.Transport,
			normalized.Endpoint, normalized.Command, normalized.Enabled, normalized.AllowedTools,
			normalized.TimeoutMS, normalized.MonthlyBudget, normalized.RedactionPolicy, metadataRaw))
		if err != nil {
			return err
		}
		config = created
		return nil
	})
	return config, err
}

func (s *Store) ListAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	logs := []AuditLog{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, user_id::text, config_id::text, tool_provider, tool_name,
				request_hash, request_summary, response_summary, latency_ms, status,
				error_message, estimated_cost::float8, resource_type, resource_id::text,
				metadata, created_at
			from external_tool_audit_logs
			where tenant_id = $1
			order by created_at desc
			limit $2
		`, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanAuditLog(rows)
			if err != nil {
				return err
			}
			logs = append(logs, item)
		}
		return rows.Err()
	})
	return logs, err
}

func (s *Store) Invoke(ctx context.Context, tenantID, userID, providerKey string, req InvokeRequest) (InvokeResult, error) {
	providerKey = normalizeProviderKey(providerKey)
	normalizedReq, err := normalizeInvokeRequest(req)
	if err != nil {
		return InvokeResult{}, err
	}
	config, spent, err := s.configForInvoke(ctx, tenantID, providerKey)
	if err != nil {
		return InvokeResult{}, err
	}
	requestHash := requestHash(normalizedReq.Arguments)
	requestSummary := summarizeArguments(normalizedReq.Arguments)
	cost := costPerCall(config.Metadata)
	if !config.Enabled {
		audit, _ := s.recordAudit(ctx, auditInput{
			TenantID: tenantID, UserID: userID, ConfigID: config.ID, ToolProvider: config.ProviderKey,
			ToolName: normalizedReq.ToolName, RequestHash: requestHash, RequestSummary: requestSummary,
			Status: StatusBlocked, ErrorMessage: "external tool provider is disabled",
			ResourceType: normalizedReq.ResourceType, ResourceID: normalizedReq.ResourceID, Metadata: map[string]any{"reason": "disabled"},
		})
		return InvokeResult{ProviderKey: config.ProviderKey, ToolName: normalizedReq.ToolName, Status: StatusBlocked, Audit: audit}, ErrInvalidRequest
	}
	if !toolAllowed(config.AllowedTools, normalizedReq.ToolName) {
		audit, _ := s.recordAudit(ctx, auditInput{
			TenantID: tenantID, UserID: userID, ConfigID: config.ID, ToolProvider: config.ProviderKey,
			ToolName: normalizedReq.ToolName, RequestHash: requestHash, RequestSummary: requestSummary,
			Status: StatusBlocked, ErrorMessage: "external tool is not in the tenant allowlist",
			ResourceType: normalizedReq.ResourceType, ResourceID: normalizedReq.ResourceID, Metadata: map[string]any{"reason": "tool_not_allowed"},
		})
		return InvokeResult{ProviderKey: config.ProviderKey, ToolName: normalizedReq.ToolName, Status: StatusBlocked, Audit: audit}, ErrInvalidRequest
	}
	if config.MonthlyBudget > 0 && spent+cost > config.MonthlyBudget {
		audit, _ := s.recordAudit(ctx, auditInput{
			TenantID: tenantID, UserID: userID, ConfigID: config.ID, ToolProvider: config.ProviderKey,
			ToolName: normalizedReq.ToolName, RequestHash: requestHash, RequestSummary: requestSummary,
			Status: StatusBlocked, ErrorMessage: "external tool monthly budget exceeded",
			EstimatedCost: cost, ResourceType: normalizedReq.ResourceType, ResourceID: normalizedReq.ResourceID,
			Metadata: map[string]any{"reason": "budget_exceeded", "monthly_budget": config.MonthlyBudget, "month_spend": spent},
		})
		return InvokeResult{ProviderKey: config.ProviderKey, ToolName: normalizedReq.ToolName, Status: StatusBlocked, Audit: audit}, ErrInvalidRequest
	}
	if config.Transport != TransportStreamableHTTP || config.Endpoint == "" {
		return InvokeResult{}, ErrInvalidRequest
	}

	started := time.Now()
	result, metadata, callErr := s.callStreamableHTTP(ctx, config, normalizedReq)
	latencyMS := int(time.Since(started).Milliseconds())
	status := StatusSuccess
	errorMessage := ""
	responseSummary := summarizeValue(result)
	if callErr != nil {
		status = StatusFailed
		errorMessage = safeError(callErr)
		responseSummary = ""
	}
	audit, auditErr := s.recordAudit(ctx, auditInput{
		TenantID: tenantID, UserID: userID, ConfigID: config.ID, ToolProvider: config.ProviderKey,
		ToolName: normalizedReq.ToolName, RequestHash: requestHash, RequestSummary: requestSummary,
		ResponseSummary: responseSummary, LatencyMS: latencyMS, Status: status, ErrorMessage: errorMessage,
		EstimatedCost: cost, ResourceType: normalizedReq.ResourceType, ResourceID: normalizedReq.ResourceID, Metadata: metadata,
	})
	if auditErr != nil {
		return InvokeResult{}, auditErr
	}
	return InvokeResult{
		ProviderKey: config.ProviderKey,
		ToolName:    normalizedReq.ToolName,
		Status:      status,
		Result:      result,
		Audit:       audit,
		Metadata:    metadata,
	}, nil
}

func (s *Store) configForInvoke(ctx context.Context, tenantID, providerKey string) (Config, float64, error) {
	var config Config
	var spent float64
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanConfig(tx.QueryRow(ctx, `
			select id::text, provider_key, name, transport, endpoint, command, enabled,
				allowed_tools, timeout_ms, monthly_budget::float8, redaction_policy,
				metadata, created_at, updated_at
			from external_tool_configs
			where tenant_id = $1 and provider_key = $2
		`, tenantID, providerKey))
		if err != nil {
			return err
		}
		config = created
		return tx.QueryRow(ctx, `
			select coalesce(sum(estimated_cost), 0)::float8
			from external_tool_audit_logs
			where tenant_id = $1
				and tool_provider = $2
				and created_at >= date_trunc('month', now())
				and status = 'success'
		`, tenantID, providerKey).Scan(&spent)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Config{}, 0, ErrNotFound
	}
	return config, spent, err
}

func (s *Store) callStreamableHTTP(ctx context.Context, config Config, req InvokeRequest) (any, map[string]any, error) {
	timeout := time.Duration(config.TimeoutMS) * time.Millisecond
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	requestID := uuid.NewString()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      req.ToolName,
			"arguments": req.Arguments,
		},
	})
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, map[string]any{"request_id": requestID}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := externalToolToken(config.ProviderKey); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, map[string]any{"request_id": requestID}, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, map[string]any{"request_id": requestID, "http_status": resp.StatusCode}, err
	}
	metadata := map[string]any{"request_id": requestID, "http_status": resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, metadata, fmt.Errorf("external tool returned HTTP %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, metadata, err
	}
	if errPayload, ok := payload["error"]; ok && errPayload != nil {
		return nil, metadata, fmt.Errorf("external tool error: %s", summarizeValue(errPayload))
	}
	if result, ok := payload["result"]; ok {
		return result, metadata, nil
	}
	return payload, metadata, nil
}

func (s *Store) recordAudit(ctx context.Context, input auditInput) (AuditLog, error) {
	input.RequestSummary = truncateSummary(input.RequestSummary)
	input.ResponseSummary = truncateSummary(input.ResponseSummary)
	input.ErrorMessage = truncateSummary(input.ErrorMessage)
	var log AuditLog
	err := s.withTenant(ctx, input.TenantID, func(tx pgx.Tx) error {
		metadataRaw, _ := json.Marshal(input.Metadata)
		created, err := scanAuditLog(tx.QueryRow(ctx, `
			insert into external_tool_audit_logs (
				tenant_id, user_id, config_id, tool_provider, tool_name,
				request_hash, request_summary, response_summary, latency_ms, status,
				error_message, estimated_cost, resource_type, resource_id, metadata
			)
			values (
				$1, nullif($2, '')::uuid, nullif($3, '')::uuid, $4, $5,
				$6, $7, $8, $9, $10,
				$11, $12, $13, nullif($14, '')::uuid, $15::jsonb
			)
			returning id::text, user_id::text, config_id::text, tool_provider, tool_name,
				request_hash, request_summary, response_summary, latency_ms, status,
				error_message, estimated_cost::float8, resource_type, resource_id::text,
				metadata, created_at
		`, input.TenantID, input.UserID, input.ConfigID, input.ToolProvider, input.ToolName,
			input.RequestHash, input.RequestSummary, input.ResponseSummary, input.LatencyMS,
			input.Status, input.ErrorMessage, input.EstimatedCost, input.ResourceType,
			input.ResourceID, metadataRaw))
		if err != nil {
			return err
		}
		log = created
		return nil
	})
	return log, err
}

func (s *Store) withTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := platformdb.WithTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func normalizeConfig(providerKey string, req UpsertConfigRequest) (Config, error) {
	providerKey = normalizeProviderKey(providerKey)
	if providerKey == "" {
		return Config{}, ErrInvalidRequest
	}
	preset, hasPreset := ProviderPresetByKey(providerKey)
	transport := strings.TrimSpace(req.Transport)
	if transport == "" {
		transport = TransportStreamableHTTP
	}
	if transport != TransportStreamableHTTP {
		return Config{}, ErrInvalidRequest
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint != "" && !validExternalToolEndpoint(endpoint) {
		return Config{}, ErrInvalidRequest
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		if hasPreset {
			name = preset.Name
		} else {
			name = providerKey
		}
	}
	enabled := false
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if enabled && endpoint == "" {
		return Config{}, ErrInvalidRequest
	}
	redactionPolicy := strings.TrimSpace(req.RedactionPolicy)
	if redactionPolicy == "" {
		redactionPolicy = "summary_only"
	}
	if redactionPolicy != "summary_only" && redactionPolicy != "no_sensitive" && redactionPolicy != "disabled" {
		return Config{}, ErrInvalidRequest
	}
	if hasPreset && redactionPolicy == "disabled" {
		return Config{}, ErrInvalidRequest
	}
	timeoutMS := req.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = defaultTimeoutMS
	}
	if timeoutMS < minTimeoutMS || timeoutMS > maxTimeoutMS || req.MonthlyBudget < 0 {
		return Config{}, ErrInvalidRequest
	}
	allowedTools := normalizeAllowedTools(req.AllowedTools)
	if len(allowedTools) == 0 && hasPreset {
		allowedTools = append([]string(nil), preset.DefaultAllowedTools...)
	}
	if hasPreset && preset.StrictAllowedTools && !allowedToolsWithin(allowedTools, preset.DefaultAllowedTools) {
		return Config{}, ErrInvalidRequest
	}
	return Config{
		ProviderKey:     providerKey,
		Name:            name,
		Transport:       transport,
		Endpoint:        endpoint,
		Command:         strings.TrimSpace(req.Command),
		Enabled:         enabled,
		AllowedTools:    allowedTools,
		TimeoutMS:       timeoutMS,
		MonthlyBudget:   req.MonthlyBudget,
		RedactionPolicy: redactionPolicy,
		Metadata:        normalizeMetadata(req.Metadata),
	}, nil
}

func normalizeInvokeRequest(req InvokeRequest) (InvokeRequest, error) {
	toolName := strings.TrimSpace(req.ToolName)
	if toolName == "" || len(toolName) > 128 {
		return InvokeRequest{}, ErrInvalidRequest
	}
	resourceID := strings.TrimSpace(req.ResourceID)
	if resourceID != "" {
		if _, err := uuid.Parse(resourceID); err != nil {
			return InvokeRequest{}, ErrInvalidRequest
		}
	}
	return InvokeRequest{
		ToolName:     toolName,
		Arguments:    normalizeMetadata(req.Arguments),
		ResourceType: strings.TrimSpace(req.ResourceType),
		ResourceID:   resourceID,
	}, nil
}

func normalizeProviderKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`).MatchString(value) {
		return ""
	}
	return value
}

func normalizeAllowedTools(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		tool := strings.TrimSpace(value)
		if tool == "" || seen[tool] {
			continue
		}
		seen[tool] = true
		result = append(result, tool)
	}
	sort.Strings(result)
	return result
}

func normalizeMetadata(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func newExternalToolHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
				if err != nil {
					return nil, err
				}
				if len(ips) == 0 {
					return nil, ErrInvalidRequest
				}
				for _, ip := range ips {
					if !publicExternalToolNetIP(ip) {
						return nil, ErrInvalidRequest
					}
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
			},
		},
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !validExternalToolEndpoint(req.URL.String()) {
				return ErrInvalidRequest
			}
			return nil
		},
	}
}

func validExternalToolEndpoint(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.User != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return false
	}
	if externalToolEndpointHasSensitiveQuery(parsed.Query()) {
		return false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return publicExternalToolNetIP(addr)
	}
	return strings.Contains(host, ".")
}

func publicExternalToolNetIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() ||
		!addr.IsGlobalUnicast() ||
		addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return false
	}
	for _, prefix := range externalToolSpecialUseIPPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var externalToolSpecialUseIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

func toolAllowed(allowed []string, tool string) bool {
	for _, item := range allowed {
		if item == tool {
			return true
		}
	}
	return false
}

func allowedToolsWithin(values []string, allowed []string) bool {
	allowedSet := map[string]bool{}
	for _, tool := range allowed {
		allowedSet[tool] = true
	}
	for _, tool := range values {
		if !allowedSet[tool] {
			return false
		}
	}
	return true
}

func requestHash(arguments map[string]any) string {
	raw, _ := json.Marshal(arguments)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func summarizeArguments(arguments map[string]any) string {
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+summarizeType(arguments[key]))
	}
	return truncateSummary(strings.Join(parts, ", "))
}

func summarizeType(value any) string {
	switch typed := value.(type) {
	case string:
		return fmt.Sprintf("string(len=%d)", len([]rune(typed)))
	case []any:
		return fmt.Sprintf("array(len=%d)", len(typed))
	case map[string]any:
		return fmt.Sprintf("object(keys=%d)", len(typed))
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func summarizeValue(value any) string {
	raw, err := json.Marshal(structuralSummary(value, 0))
	if err != nil {
		return summarizeType(value)
	}
	return truncateSummary(string(raw))
}

func structuralSummary(value any, depth int) any {
	if depth >= 4 {
		return summarizeType(value)
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		summary := map[string]any{
			"type":      "object",
			"key_count": len(keys),
		}
		if len(keys) > 0 {
			visibleKeys := keys
			if len(visibleKeys) > 12 {
				visibleKeys = visibleKeys[:12]
			}
			summary["keys"] = visibleKeys
			for _, key := range visibleKeys {
				summary[key] = structuralSummary(typed[key], depth+1)
			}
		}
		return summary
	case []any:
		return map[string]any{
			"type":  "array",
			"count": len(typed),
		}
	case string:
		return map[string]any{
			"type":   "string",
			"length": len([]rune(typed)),
		}
	case nil:
		return map[string]any{"type": "null"}
	case bool:
		return map[string]any{"type": "bool"}
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{"type": fmt.Sprintf("%T", value)}
	}
}

func truncateSummary(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxSummaryRunes {
		return string(runes)
	}
	return string(runes[:maxSummaryRunes])
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	return truncateSummary(redactExternalToolError(err.Error()))
}

func redactExternalToolError(value string) string {
	message := externalToolURLPattern.ReplaceAllString(value, "[external-endpoint]")
	message = externalToolSecretPattern.ReplaceAllString(message, "$1=[redacted]")
	return message
}

func costPerCall(metadata map[string]any) float64 {
	value, ok := metadata["cost_per_call"]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return float64(typed)
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(typed, "%f", &parsed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func externalToolToken(providerKey string) string {
	return strings.TrimSpace(os.Getenv(externalToolTokenEnvName(providerKey)))
}

func externalToolTokenEnvName(providerKey string) string {
	return "EXTERNAL_TOOL_" + strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(providerKey)) + "_TOKEN"
}

func externalToolEndpointHasSensitiveQuery(query url.Values) bool {
	for key := range query {
		if externalToolSensitiveQueryKeys[strings.ToLower(strings.TrimSpace(key))] {
			return true
		}
	}
	return false
}

var externalToolSensitiveQueryKeys = map[string]bool{
	"access_token":  true,
	"apikey":        true,
	"api_key":       true,
	"authorization": true,
	"auth_token":    true,
	"key":           true,
	"password":      true,
	"secret":        true,
	"sig":           true,
	"signature":     true,
	"token":         true,
}

var (
	externalToolURLPattern    = regexp.MustCompile(`https?://[^\s"'<>]+`)
	externalToolSecretPattern = regexp.MustCompile(`(?i)\b(token|access_token|auth_token|api[_-]?key|secret|password|authorization|signature|sig)\s*=\s*[^,\s&;]+`)
)

func scanConfig(row pgx.Row) (Config, error) {
	var item Config
	var metadataRaw []byte
	err := row.Scan(
		&item.ID, &item.ProviderKey, &item.Name, &item.Transport, &item.Endpoint, &item.Command,
		&item.Enabled, &item.AllowedTools, &item.TimeoutMS, &item.MonthlyBudget, &item.RedactionPolicy,
		&metadataRaw, &item.CreatedAt, &item.UpdatedAt,
	)
	item.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
	}
	return item, err
}

func scanAuditLog(row pgx.Row) (AuditLog, error) {
	var item AuditLog
	var userID, configID, resourceID sql.NullString
	var metadataRaw []byte
	err := row.Scan(
		&item.ID, &userID, &configID, &item.ToolProvider, &item.ToolName,
		&item.RequestHash, &item.RequestSummary, &item.ResponseSummary, &item.LatencyMS,
		&item.Status, &item.ErrorMessage, &item.EstimatedCost, &item.ResourceType,
		&resourceID, &metadataRaw, &item.CreatedAt,
	)
	if userID.Valid {
		item.UserID = &userID.String
	}
	if configID.Valid {
		item.ConfigID = &configID.String
	}
	if resourceID.Valid {
		item.ResourceID = &resourceID.String
	}
	item.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
	}
	return item, err
}
