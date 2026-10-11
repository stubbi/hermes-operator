package secretsbroker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testToken  = "tok-SUPER-SECRET-123"
	testSecret = "sk-live-VALUE-never-log-me"
)

func envLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

// noSleep records backoff waits without sleeping.
func noSleep(waits *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return nil
	}
}

func newResolver(t *testing.T, logs *bytes.Buffer, waits *[]time.Duration) *Resolver {
	t.Helper()
	return &Resolver{
		Client: &http.Client{},
		Sleep:  noSleep(waits),
		Log:    func(f string, a ...any) { _, _ = fmt.Fprintf(logs, f+"\n", a...) },
	}
}

func TestResolve_SuccessInterpolatesURLAndHeaders(t *testing.T) {
	var gotPath, gotAuth, gotEmp string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotEmp = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Employee-Id")
		_, _ = w.Write([]byte(`{"OPENAI_API_KEY":"` + testSecret + `","GITHUB_TOKEN":"gh"}`))
	}))
	defer srv.Close()

	var logs bytes.Buffer
	var waits []time.Duration
	r := newResolver(t, &logs, &waits)
	r.Client = srv.Client()
	cfg := &Config{
		URL:     srv.URL + "/secrets/resolve/${EMPLOYEE_ID}",
		Method:  http.MethodGet,
		Headers: map[string]string{"Authorization": "Bearer ${BROKER_AUTH_TOKEN}", "X-Employee-Id": "${EMPLOYEE_ID}"},
	}
	got, err := r.Resolve(context.Background(), cfg, envLookup(map[string]string{"EMPLOYEE_ID": "emp-7", "BROKER_AUTH_TOKEN": testToken}))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"OPENAI_API_KEY": testSecret, "GITHUB_TOKEN": "gh"}, got)
	assert.Equal(t, "/secrets/resolve/emp-7", gotPath)
	assert.Equal(t, "Bearer "+testToken, gotAuth)
	assert.Equal(t, "emp-7", gotEmp)
	assert.Empty(t, waits)
}

func TestResolve_POSTAndPrefix(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		_, _ = w.Write([]byte(`{"KEY":"v"}`))
	}))
	defer srv.Close()
	var logs bytes.Buffer
	var waits []time.Duration
	r := newResolver(t, &logs, &waits)
	got, err := r.Resolve(context.Background(), &Config{URL: srv.URL, Method: http.MethodPost, EnvPrefix: "BROKER_", AllowInsecureHTTP: true}, envLookup(nil))
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, map[string]string{"BROKER_KEY": "v"}, got)
}

func TestResolve_FailClosedNotRetried(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusBadRequest} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(code)
				// A misbehaving broker echoing secrets in an error body must not leak.
				_, _ = w.Write([]byte(`{"OPENAI_API_KEY":"` + testSecret + `"}`))
			}))
			defer srv.Close()
			var logs bytes.Buffer
			var waits []time.Duration
			r := newResolver(t, &logs, &waits)
			got, err := r.Resolve(context.Background(),
				&Config{URL: srv.URL + "/r?emp=7", AllowInsecureHTTP: true, MaxAttempts: 5, Headers: map[string]string{"Authorization": "Bearer ${T}"}},
				envLookup(map[string]string{"T": testToken}))
			require.Error(t, err)
			assert.Nil(t, got, "no partial or empty map on failure")
			var se *StatusError
			require.True(t, errors.As(err, &se))
			assert.Equal(t, code, se.Code)
			assert.Equal(t, int32(1), calls.Load(), "client errors are not retried")
			assertNoLeak(t, err.Error()+logs.String())
			assert.NotContains(t, err.Error(), "emp=7", "query string is redacted")
		})
	}
	assert.Contains(t, (&StatusError{Code: 401, URL: "u"}).Error(), "credentials")
	assert.Contains(t, (&StatusError{Code: 404, URL: "u"}).Error(), "no active secrets")
}

func TestResolve_RetriesTransientThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"A":"b"}`))
	}))
	defer srv.Close()
	var logs bytes.Buffer
	var waits []time.Duration
	r := newResolver(t, &logs, &waits)
	got, err := r.Resolve(context.Background(), &Config{URL: srv.URL, AllowInsecureHTTP: true}, envLookup(nil))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "b"}, got)
	assert.Equal(t, int32(3), calls.Load())
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, waits, "bounded exponential backoff")
}

func TestResolve_RetriesExhausted(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	var logs bytes.Buffer
	var waits []time.Duration
	r := newResolver(t, &logs, &waits)
	_, err := r.Resolve(context.Background(), &Config{URL: srv.URL, AllowInsecureHTTP: true, MaxAttempts: 2}, envLookup(nil))
	require.Error(t, err)
	assert.Equal(t, int32(2), calls.Load())
}

func TestResolve_Timeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	var logs bytes.Buffer
	var waits []time.Duration
	r := newResolver(t, &logs, &waits)
	start := time.Now()
	_, err := r.Resolve(context.Background(), &Config{URL: srv.URL, AllowInsecureHTTP: true, TimeoutSeconds: 1, MaxAttempts: 2}, envLookup(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
	assert.Len(t, waits, 1, "a timeout is transient and retried")
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestResolve_RejectsBadBodies(t *testing.T) {
	cases := map[string]string{
		"not json":         `oops ` + testSecret,
		"non-string":       `{"A":1}`,
		"array":            `["` + testSecret + `"]`,
		"invalid name":     `{"BAD-NAME":"` + testSecret + `"}`,
		"protected PATH":   `{"PATH":"/evil"}`,
		"protected LD_":    `{"LD_PRELOAD":"/evil.so"}`,
		"protected HERMES": `{"HERMES_HOME":"/tmp"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			var logs bytes.Buffer
			var waits []time.Duration
			r := newResolver(t, &logs, &waits)
			got, err := r.Resolve(context.Background(), &Config{URL: srv.URL, AllowInsecureHTTP: true}, envLookup(nil))
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Equal(t, int32(1), calls.Load(), "a malformed answer is not retried")
			assertNoLeak(t, err.Error()+logs.String())
		})
	}
}

func TestResolve_RefusesPlainHTTPByDefault(t *testing.T) {
	var logs bytes.Buffer
	var waits []time.Duration
	r := newResolver(t, &logs, &waits)
	_, err := r.Resolve(context.Background(), &Config{URL: "http://broker.local/x"}, envLookup(nil))
	require.ErrorContains(t, err, "plain http")
	_, err = r.Resolve(context.Background(), &Config{URL: "ftp://broker.local/x"}, envLookup(nil))
	require.ErrorContains(t, err, "scheme")
}

func TestInterpolate(t *testing.T) {
	got, err := Interpolate("a/${X}/b/${Y}", envLookup(map[string]string{"X": "1", "Y": "2"}))
	require.NoError(t, err)
	assert.Equal(t, "a/1/b/2", got)

	_, err = Interpolate("Bearer ${TOKEN}", envLookup(map[string]string{"TOKEN": ""}))
	require.ErrorContains(t, err, "TOKEN")
	_, err = Interpolate("${A}${B}", envLookup(nil))
	require.ErrorContains(t, err, "A, B")

	got, err = Interpolate("literal $X and $", envLookup(nil))
	require.NoError(t, err)
	assert.Equal(t, "literal $X and $", got, "only ${VAR} is a placeholder")
}

func TestParseConfig(t *testing.T) {
	c, err := ParseConfig(`{"url":"https://b/x","headers":{"A":"b"}}`)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, c.Method)

	c, err = ParseConfig(`{"url":"https://b/x","method":"post"}`)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, c.Method)

	for _, bad := range []string{``, `{}`, `{"url":"https://b","method":"PUT"}`, `{"url":"https://b","extra":1}`, `{"url":"https://b","envPrefix":"1X"}`, `{"url":"https://b","maxAttempts":-1}`} {
		_, err := ParseConfig(bad)
		assert.Error(t, err, bad)
	}
}

func TestMergeEnv(t *testing.T) {
	env := []string{"PATH=/bin", "OPENAI_API_KEY=placeholder", ConfigEnv + `={"url":"x"}`, "EMPTY="}
	got := MergeEnv(env, map[string]string{"OPENAI_API_KEY": "real", "B": "2"})
	assert.Equal(t, []string{"PATH=/bin", "EMPTY=", "B=2", "OPENAI_API_KEY=real"}, got)
}

func TestExec_ResolvesAndExecsWithSecrets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"OPENAI_API_KEY":"` + testSecret + `"}`))
	}))
	defer srv.Close()
	environ := []string{
		ConfigEnv + `={"url":"` + srv.URL + `/r/${EMP}","allowInsecureHTTP":true,"headers":{"Authorization":"Bearer ${TOK}"}}`,
		"EMP=7", "TOK=" + testToken,
	}
	var logs bytes.Buffer
	var argv0 string
	var gotArgv, gotEnv []string
	execve := func(a0 string, argv, envv []string) error {
		argv0, gotArgv, gotEnv = a0, argv, envv
		return nil
	}
	r := &Resolver{Log: func(f string, a ...any) { _, _ = fmt.Fprintf(&logs, f+"\n", a...) }}
	err := Exec(context.Background(), []string{"/init", "/wrapper.sh", "gateway", "run"}, environ, r, execve)
	require.NoError(t, err)
	assert.Equal(t, "/init", argv0)
	assert.Equal(t, []string{"/init", "/wrapper.sh", "gateway", "run"}, gotArgv)
	assert.Contains(t, gotEnv, "OPENAI_API_KEY="+testSecret)
	for _, kv := range gotEnv {
		assert.False(t, strings.HasPrefix(kv, ConfigEnv+"="), "broker config is not passed to the agent")
	}
	assert.Contains(t, logs.String(), "OPENAI_API_KEY", "names are logged")
	assertNoLeak(t, logs.String())
}

func TestExec_FailsClosedWithoutExec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	called := false
	execve := func(string, []string, []string) error { called = true; return nil }
	err := Exec(context.Background(), []string{"/init"},
		[]string{ConfigEnv + `={"url":"` + srv.URL + `","allowInsecureHTTP":true}`}, &Resolver{}, execve)
	require.Error(t, err)
	assert.False(t, called, "the agent must not start without its secrets")

	err = Exec(context.Background(), []string{"/init"}, nil, &Resolver{}, execve)
	require.ErrorContains(t, err, ConfigEnv)
	assert.False(t, called)
}

func TestMain_Usage(t *testing.T) {
	var buf bytes.Buffer
	assert.Equal(t, 2, Main(nil, &buf))
	assert.Equal(t, 2, Main([]string{"nope"}, &buf))
	assert.Equal(t, 2, Main([]string{"exec"}, &buf))
	assert.Equal(t, 2, Main([]string{"install"}, &buf))
}

func TestInstall_CopiesExecutable(t *testing.T) {
	dest := t.TempDir() + "/sub/resolver"
	require.NoError(t, Install(dest))
	st, err := statMode(dest)
	require.NoError(t, err)
	assert.Equal(t, "-rwxr-xr-x", st)
}

func assertNoLeak(t *testing.T, s string) {
	t.Helper()
	assert.NotContains(t, s, testSecret, "secret value leaked")
	assert.NotContains(t, s, testToken, "authorization token leaked")
}

func statMode(p string) (string, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	return fi.Mode().String(), nil
}
