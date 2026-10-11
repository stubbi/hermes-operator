// Package secretsbroker resolves short-lived secret material for the agent
// from an external secrets broker at container start.
//
// The operator copies its own binary into the agent pod (see Install) and runs
// it as the agent container's entrypoint wrapper (see Exec). The wrapper reads
// the broker configuration from the HERMES_SECRETS_BROKER_CONFIG env var (the
// `secrets_broker` block shape: url, headers, plus a few knobs), interpolates
// ${VAR} placeholders from the container env, fetches a JSON map of secret
// name to plaintext value, and then execve(2)s the image's real entrypoint
// with those secrets added to the environment.
//
// Design constraints:
//
//   - Fail closed. A non-2xx answer, a malformed body, or an invalid secret
//     name aborts the start; there is no fallback to stale or empty values.
//     401/403/404 are not retried because they will not get better by retrying.
//   - Never log or return secret values. Errors and logs carry the secret
//     names and HTTP status only; header values are never printed.
//   - No persistence. The secrets live in the process environment of the
//     exec'd entrypoint only: nothing is written to the PVC, so they never
//     reach snapshots or backups. Every container (re)start resolves afresh,
//     which is how rotation and revocation take effect.
//   - PID 1 is preserved. execve replaces the wrapper in place, so s6-overlay's
//     /init still runs as PID 1.
package secretsbroker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ConfigEnv is the env var the operator renders the broker config into.
const ConfigEnv = "HERMES_SECRETS_BROKER_CONFIG"

// Defaults applied when the config leaves a knob unset.
const (
	DefaultTimeout     = 10 * time.Second
	DefaultMaxAttempts = 3
	// maxBodyBytes bounds the broker response. A secrets map is small; a huge
	// body is a misbehaving broker, not a reason to exhaust memory.
	maxBodyBytes = 1 << 20
)

// Config is the secrets_broker block. JSON tags match the block shape proposed
// in the issue (url, headers) so the same document works whether it is
// rendered by the operator from spec.secretsBroker or written by hand.
type Config struct {
	// URL of the resolve endpoint. May contain ${VAR} placeholders.
	URL string `json:"url"`
	// Method is GET (default) or POST.
	Method string `json:"method,omitempty"`
	// Headers sent with the request. Values may contain ${VAR} placeholders.
	Headers map[string]string `json:"headers,omitempty"`
	// TimeoutSeconds bounds each attempt. Defaults to 10.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// MaxAttempts bounds retries on transient failures. Defaults to 3.
	MaxAttempts int `json:"maxAttempts,omitempty"`
	// EnvPrefix is prepended to every secret name when exported.
	EnvPrefix string `json:"envPrefix,omitempty"`
	// AllowInsecureHTTP permits a plain http:// URL (in-cluster brokers).
	AllowInsecureHTTP bool `json:"allowInsecureHTTP,omitempty"`
}

// ParseConfig decodes and validates a broker config document.
func ParseConfig(raw string) (*Config, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s is empty", ConfigEnv)
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("decode %s: %w", ConfigEnv, err)
	}
	if c.URL == "" {
		return nil, errors.New("secrets broker: url is required")
	}
	switch strings.ToUpper(c.Method) {
	case "":
		c.Method = http.MethodGet
	case http.MethodGet, http.MethodPost:
		c.Method = strings.ToUpper(c.Method)
	default:
		return nil, fmt.Errorf("secrets broker: method %q is not supported (use GET or POST)", c.Method)
	}
	if c.TimeoutSeconds < 0 || c.MaxAttempts < 0 {
		return nil, errors.New("secrets broker: timeoutSeconds and maxAttempts must not be negative")
	}
	if c.EnvPrefix != "" && !envNameRE.MatchString(c.EnvPrefix) {
		return nil, fmt.Errorf("secrets broker: envPrefix %q is not a valid env var name prefix", c.EnvPrefix)
	}
	return &c, nil
}

var (
	placeholderRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	envNameRE     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Interpolate replaces every ${VAR} in s with the value lookup returns. An
// unset or empty variable is an error: sending "Bearer " or an URL with an
// empty employee id would turn a configuration mistake into a confusing 401.
func Interpolate(s string, lookup func(string) (string, bool)) (string, error) {
	var missing []string
	out := placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		name := placeholderRE.FindStringSubmatch(m)[1]
		v, ok := lookup(name)
		if !ok || v == "" {
			missing = append(missing, name)
			return ""
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("secrets broker: placeholder(s) %s are unset or empty in the container env", strings.Join(missing, ", "))
	}
	return out, nil
}

// StatusError is a non-2xx broker answer. It never carries the body, which
// could echo secret material.
type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string {
	hint := ""
	switch e.Code {
	case http.StatusUnauthorized, http.StatusForbidden:
		hint = ": check the broker credentials (Authorization header source)"
	case http.StatusNotFound:
		hint = ": the broker has no active secrets for this identity (revoked, or a wrong employee/workspace id)"
	}
	return fmt.Sprintf("secrets broker: %s answered HTTP %d%s", e.URL, e.Code, hint)
}

// retryable reports whether a failure is worth another attempt.
func retryable(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Code == http.StatusTooManyRequests || se.Code >= 500
	}
	var de *decodeError
	return !errors.As(err, &de)
}

type decodeError struct{ msg string }

func (e *decodeError) Error() string { return e.msg }

// Resolver fetches secrets from a broker.
type Resolver struct {
	Client *http.Client
	// Sleep is the backoff wait; replaced in tests.
	Sleep func(context.Context, time.Duration) error
	// Log receives progress lines. It never sees secret values.
	Log func(format string, args ...any)
}

func (r *Resolver) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Resolve interpolates the config against lookup and fetches the secrets map.
// The returned map is keyed by the exported env var name (prefix applied).
func (r *Resolver) Resolve(ctx context.Context, c *Config, lookup func(string) (string, bool)) (map[string]string, error) {
	rawURL, err := Interpolate(c.URL, lookup)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("secrets broker: url %q is not a valid absolute URL", c.URL)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !c.AllowInsecureHTTP {
			return nil, errors.New("secrets broker: refusing a plain http:// url; use https or set allowInsecureHTTP for an in-cluster broker")
		}
	default:
		return nil, fmt.Errorf("secrets broker: unsupported url scheme %q", u.Scheme)
	}
	headers := make(http.Header, len(c.Headers))
	for k, v := range c.Headers {
		iv, err := Interpolate(v, lookup)
		if err != nil {
			return nil, fmt.Errorf("header %s: %w", k, err)
		}
		headers.Set(k, iv)
	}

	timeout := DefaultTimeout
	if c.TimeoutSeconds > 0 {
		timeout = time.Duration(c.TimeoutSeconds) * time.Second
	}
	attempts := DefaultMaxAttempts
	if c.MaxAttempts > 0 {
		attempts = c.MaxAttempts
	}
	client := r.Client
	if client == nil {
		client = &http.Client{}
	}
	sleep := r.Sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	// Log a redacted target: scheme, host and path only. Query strings and
	// header values may carry identity or credentials.
	target := u.Scheme + "://" + u.Host + u.EscapedPath()

	var lastErr error
	for i := 1; i <= attempts; i++ {
		var secrets map[string]string
		secrets, lastErr = r.fetch(ctx, client, c.Method, u.String(), target, headers, timeout)
		if lastErr == nil {
			return applyPrefix(secrets, c.EnvPrefix)
		}
		if !retryable(lastErr) || i == attempts || ctx.Err() != nil {
			break
		}
		backoff := time.Duration(1<<(i-1)) * time.Second
		r.logf("secrets broker: attempt %d/%d failed (%v); retrying in %s", i, attempts, lastErr, backoff)
		if err := sleep(ctx, backoff); err != nil {
			return nil, fmt.Errorf("secrets broker: %w (after %v)", err, lastErr)
		}
	}
	return nil, lastErr
}

func (r *Resolver) fetch(ctx context.Context, client *http.Client, method, rawURL, target string, headers http.Header, timeout time.Duration) (map[string]string, error) {
	actx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(actx, method, rawURL, nil)
	if err != nil {
		return nil, &decodeError{fmt.Sprintf("secrets broker: build request: %v", err)}
	}
	req.Header = headers.Clone()
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		// url.Error embeds the full URL; report the redacted target instead.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("secrets broker: request to %s failed: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))
		return nil, &StatusError{Code: resp.StatusCode, URL: target}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("secrets broker: read response from %s: %w", target, err)
	}
	if len(body) > maxBodyBytes {
		return nil, &decodeError{fmt.Sprintf("secrets broker: response from %s exceeds %d bytes", target, maxBodyBytes)}
	}
	var out map[string]string
	if err := json.Unmarshal(body, &out); err != nil {
		// Do not wrap the json error: it can quote fragments of the body.
		return nil, &decodeError{fmt.Sprintf("secrets broker: response from %s is not a JSON object of string values", target)}
	}
	return out, nil
}

// protectedEnv are names a broker must never override: they steer the
// runtime, the s6 privilege drop, or the dynamic loader.
var protectedEnv = map[string]struct{}{
	"PATH": {}, "HOME": {}, "USER": {}, "SHELL": {}, "PWD": {},
	"HERMES_HOME": {}, "HERMES_UID": {}, "HERMES_GID": {},
	"API_SERVER_ENABLED": {}, "API_SERVER_HOST": {}, "API_SERVER_PORT": {}, "API_SERVER_KEY": {},
	ConfigEnv: {},
}

var protectedPrefixes = []string{"LD_", "S6_", "PYTHON", "SSL_CERT_"}

func applyPrefix(in map[string]string, prefix string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	var bad []string
	for k, v := range in {
		name := prefix + k
		if !envNameRE.MatchString(name) || isProtected(name) {
			bad = append(bad, k)
			continue
		}
		out[name] = v
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return nil, &decodeError{fmt.Sprintf("secrets broker: refusing secret name(s) %s: not a valid env var name or a protected runtime variable", strings.Join(bad, ", "))}
	}
	return out, nil
}

func isProtected(name string) bool {
	if _, ok := protectedEnv[name]; ok {
		return true
	}
	for _, p := range protectedPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// MergeEnv returns environ with secrets set, replacing existing entries of the
// same name and dropping the broker config itself (it may reference
// credentials the agent has no need for).
func MergeEnv(environ []string, secrets map[string]string) []string {
	out := make([]string, 0, len(environ)+len(secrets))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if _, ok := secrets[name]; ok || name == ConfigEnv {
			continue
		}
		out = append(out, kv)
	}
	names := make([]string, 0, len(secrets))
	for k := range secrets {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		out = append(out, k+"="+secrets[k])
	}
	return out
}

// SortedNames returns the secret names for logging.
func SortedNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
