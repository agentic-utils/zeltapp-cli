package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultBaseURL = "https://go.zelt.app"

type session struct {
	Email        string    `json:"email"`
	UserID       int       `json:"userId"`
	CompanyID    int       `json:"companyId"`
	DisplayName  string    `json:"displayName"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refreshToken"`
	SavedAt      time.Time `json:"savedAt"`
}

// store is the persistence interface (session on disk, password in keychain).
// Pluggable so tests can use an in-memory implementation.
type store interface {
	LoadSession() (*session, error)
	SaveSession(*session) error
	ClearSession() error
	GetPassword(email string) (string, error)
	SetPassword(email, password string) error
	DeletePassword(email string) error
}

type fileStore struct {
	dir         string
	keychainCmd string // path to `security` (macOS); empty disables keychain
}

func defaultStore() *fileStore {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	keychain := ""
	if info, err := os.Stat("/usr/bin/security"); err == nil && info.Mode()&0o111 != 0 {
		keychain = "/usr/bin/security"
	}
	return &fileStore{
		dir:         filepath.Join(dir, "zeltapp-cli"),
		keychainCmd: keychain,
	}
}

func (s *fileStore) sessionPath() string { return filepath.Join(s.dir, "session.json") }

func (s *fileStore) LoadSession() (*session, error) {
	b, err := os.ReadFile(s.sessionPath())
	if err != nil {
		return nil, err
	}
	var sess session
	if err := json.Unmarshal(b, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *fileStore) SaveSession(sess *session) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	sess.SavedAt = time.Now()
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.sessionPath(), b, 0o600)
}

func (s *fileStore) ClearSession() error {
	err := os.Remove(s.sessionPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// loadSession / saveSession / clearSession compat shims used by older command files.
func loadSession() (*session, error)    { return defaultStore().LoadSession() }
func saveSession(s *session) error      { return defaultStore().SaveSession(s) }
func clearSession() error               { return defaultStore().ClearSession() }
func sessionPath() string               { return defaultStore().sessionPath() }

type client struct {
	http    *http.Client
	jar     http.CookieJar
	baseURL string
	now     func() time.Time
	tz      func() string
	session *session
	store   store
	verbose bool
}

type clientOpt func(*client)

func withBaseURL(u string) clientOpt        { return func(c *client) { c.baseURL = u } }
func withHTTP(h *http.Client) clientOpt     { return func(c *client) { c.http = h } }
func withStore(s store) clientOpt           { return func(c *client) { c.store = s } }
func withNow(f func() time.Time) clientOpt  { return func(c *client) { c.now = f } }
func withVerbose(v bool) clientOpt          { return func(c *client) { c.verbose = v } }

func newClient(opts ...clientOpt) (*client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &client{
		jar:     jar,
		baseURL: defaultBaseURL,
		now:     time.Now,
		tz:      localTimezone,
		store:   defaultStore(),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.http == nil {
		c.http = &http.Client{Jar: c.jar, Timeout: 30 * time.Second}
	} else if c.http.Jar == nil {
		c.http.Jar = c.jar
	} else {
		c.jar = c.http.Jar
	}
	if s, err := c.store.LoadSession(); err == nil {
		c.session = s
		c.setCookies(s.Token, s.RefreshToken)
	}
	return c, nil
}

// newClientFromFlags is the constructor used by the cobra commands.
func newClientFromFlags(verbose bool) (*client, error) {
	return newClient(withVerbose(verbose))
}

func (c *client) setCookies(token, refresh string) {
	u, _ := url.Parse(c.baseURL)
	cookies := []*http.Cookie{}
	if token != "" {
		cookies = append(cookies, &http.Cookie{Name: "token", Value: token, Path: "/"})
	}
	if refresh != "" {
		cookies = append(cookies, &http.Cookie{Name: "refresh_token", Value: refresh, Path: "/"})
	}
	c.jar.SetCookies(u, cookies)
}

func (c *client) extractCookies(resp *http.Response) (token, refresh string) {
	for _, ck := range resp.Cookies() {
		switch ck.Name {
		case "token":
			token = ck.Value
		case "refresh_token":
			refresh = ck.Value
		}
	}
	return
}

type apiError struct {
	Status int
	Method string
	URL    string
	Body   string
}

func (e *apiError) Error() string {
	// Try to extract a friendly message from the body. Zelt errors look like
	// {"message":"Forbidden","statusCode":403,...}.
	msg := extractAPIMessage(e.Body)
	if msg != "" {
		return fmt.Sprintf("%d %s %s: %s", e.Status, e.Method, e.URL, msg)
	}
	// Fall back to truncated raw body.
	body := strings.TrimSpace(e.Body)
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	return fmt.Sprintf("%d %s %s: %s", e.Status, e.Method, e.URL, body)
}

// extractAPIMessage pulls a human message out of a Zelt error body if present.
func extractAPIMessage(body string) string {
	var v struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return ""
	}
	if v.Message != "" {
		return v.Message
	}
	return v.Error
}

var errUnauthorized = errors.New("unauthorized; run `zeltapp login`")

func (c *client) do(method, path string, body any, out any) error {
	// Cache lookup for cacheable GETs.
	if method == "GET" && body == nil {
		key := stripQuery(path)
		if ttl := cacheTTLFor(key); ttl > 0 {
			if raw, ok := cacheGet(key); ok {
				if c.verbose {
					fmt.Fprintf(os.Stderr, "* %s %s (cache hit)\n", method, path)
				}
				if out != nil {
					return json.Unmarshal(raw, out)
				}
				return nil
			}
		}
	}
	err := c.doWithRetry(method, path, body, out, true)
	// Cache write on success.
	if err == nil && method == "GET" && body == nil && out != nil {
		key := stripQuery(path)
		if ttl := cacheTTLFor(key); ttl > 0 {
			if raw, mErr := json.Marshal(out); mErr == nil {
				_ = cacheSet(key, raw, ttl)
			}
		}
	}
	return err
}

func stripQuery(p string) string {
	if i := strings.IndexByte(p, '?'); i >= 0 {
		return p[:i]
	}
	return p
}

func (c *client) doWithRetry(method, path string, body any, out any, retry bool) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/")
	req.Header.Set("User-Agent", "zeltapp-cli/0.1 (+github.com/brtkwr/zeltapp-cli)")
	req.Header.Set("X-Timezone", c.tz())
	req.Header.Set("X-Now-String", c.now().Format("2006-01-02 15:04:05"))

	if c.verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, path)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if c.verbose {
		fmt.Fprintf(os.Stderr, "< %d %s (%d bytes)\n", resp.StatusCode, resp.Status, len(rb))
	}

	if resp.StatusCode == 401 && retry && c.session != nil {
		if err := c.reauth(); err != nil {
			return errUnauthorized
		}
		return c.doWithRetry(method, path, body, out, false)
	}

	if token, refresh := c.extractCookies(resp); token != "" {
		if c.session == nil {
			c.session = &session{}
		}
		c.session.Token = token
		if refresh != "" {
			c.session.RefreshToken = refresh
		}
		_ = c.store.SaveSession(c.session)
	}

	if resp.StatusCode >= 400 {
		return &apiError{Status: resp.StatusCode, Method: method, URL: path, Body: string(rb)}
	}
	if out != nil && len(rb) > 0 {
		return json.Unmarshal(rb, out)
	}
	return nil
}

// mfaPromptHook is the function used by reauth() to obtain an MFA code.
// Production sets this to promptMFA; tests override it.
var mfaPromptHook mfaPrompter = promptMFA

// reauth tries refresh_token first, then falls back to stored creds + interactive MFA.
func (c *client) reauth() error {
	if c.session != nil && c.session.RefreshToken != "" {
		if err := c.tryRefresh(); err == nil {
			return nil
		}
	}
	if c.session == nil || c.session.Email == "" {
		return errors.New("no session to refresh")
	}
	pw, err := c.store.GetPassword(c.session.Email)
	if err != nil || pw == "" {
		return errors.New("refresh failed and no stored password; run `zeltapp login`")
	}
	return c.passwordLogin(c.session.Email, pw, mfaPromptHook)
}

func (c *client) tryRefresh() error {
	candidates := []string{"/apiv2/auth/refresh", "/apiv2/auth/refresh-token", "/apiv2/auth/token/refresh"}
	for _, path := range candidates {
		req, _ := http.NewRequest("POST", c.baseURL+path, nil)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			continue
		}
		if token, refresh := c.extractCookies(resp); token != "" {
			c.session.Token = token
			if refresh != "" {
				c.session.RefreshToken = refresh
			}
			return c.store.SaveSession(c.session)
		}
		var b struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.Unmarshal(body, &b); err == nil && b.AccessToken != "" {
			c.session.Token = b.AccessToken
			if b.RefreshToken != "" {
				c.session.RefreshToken = b.RefreshToken
			}
			c.setCookies(c.session.Token, c.session.RefreshToken)
			return c.store.SaveSession(c.session)
		}
	}
	return errors.New("refresh failed")
}

type mfaPrompter func(method string) (string, error)

// passwordLogin performs a fresh login using stored creds. promptMFA is called
// once if the server demands an MFA code.
func (c *client) passwordLogin(email, password string, prompt mfaPrompter) error {
	body := map[string]any{"username": email, "password": password}
	var step1 struct {
		MFAType string `json:"mfaType"`
	}
	if err := c.doWithRetry("POST", "/apiv2/auth/login", body, &step1, false); err != nil {
		return err
	}
	if step1.MFAType != "" {
		code, err := prompt(step1.MFAType)
		if err != nil {
			return err
		}
		body["mfaCode"] = code
	}
	var step2 struct {
		AccessToken string `json:"accessToken"`
		UserID      int    `json:"userId"`
		CompanyID   int    `json:"companyId"`
		DisplayName string `json:"displayName"`
	}
	if err := c.doWithRetry("POST", "/apiv2/auth/login", body, &step2, false); err != nil {
		return err
	}
	// doWithRetry has already populated c.session.Token/RefreshToken from Set-Cookie
	// headers. Fill in identity fields and ensure a token (some servers only return
	// the access token in the body, not as a cookie).
	if c.session == nil {
		c.session = &session{}
	}
	c.session.Email = email
	c.session.UserID = step2.UserID
	c.session.CompanyID = step2.CompanyID
	c.session.DisplayName = step2.DisplayName
	if c.session.Token == "" {
		c.session.Token = step2.AccessToken
	}
	return c.store.SaveSession(c.session)
}

// promptMFA reads an MFA code from stdin (used in production reauth flow).
func promptMFA(method string) (string, error) {
	fmt.Fprintf(os.Stderr, "MFA code (%s): ", method)
	var line string
	_, err := fmt.Fscanln(os.Stdin, &line)
	return strings.TrimSpace(line), err
}

func (c *client) requireSession() error {
	if c.session == nil || c.session.Token == "" {
		return errors.New("not logged in; run `zeltapp login`")
	}
	return nil
}

func localTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	name, _ := time.Now().Zone()
	if name == "" {
		return "UTC"
	}
	return name
}

func renderJSON(v any) error {
	enc := json.NewEncoder(outWriter)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func renderRaw(raw json.RawMessage) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		_, err := outWriter.Write(raw)
		return err
	}
	if flagJSON {
		return renderJSON(v)
	}
	return renderHuman(v)
}

func errBody(err error) string {
	// apiError already formats nicely via its Error() method.
	return err.Error()
}
