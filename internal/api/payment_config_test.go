package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAlipayTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "alipay.local.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func noAlipayEnv(string) (string, bool) { return "", false }

func TestAlipayLocalFileConfig(t *testing.T) {
	path := writeAlipayTestConfig(t, "\xef\xbb\xbf"+`{
		"appId": " file-app ",
		"privateKey": "-----BEGIN PRIVATE KEY-----\nlocal-test-key\n-----END PRIVATE KEY-----",
		"publicKey": " file-public ",
		"production": true,
		"notifyUrl": " https://example.com/api/pay/alipay/notify "
	}`)
	cfg, ready, err := readAlipayConfig(path, true, noAlipayEnv)
	if err != nil {
		t.Fatal(err)
	}
	if !ready || cfg.appID != "file-app" || cfg.publicKey != "file-public" || !cfg.production || cfg.notifyURL != "https://example.com/api/pay/alipay/notify" || !strings.Contains(cfg.privateKey, "\nlocal-test-key\n") {
		t.Fatal("local file values, whitespace or multiline keys were not loaded correctly")
	}
}

func TestAlipayEnvironmentOverridesLocalFile(t *testing.T) {
	path := writeAlipayTestConfig(t, `{"appId":"file-app","privateKey":"file-private","publicKey":"file-public","production":true,"notifyUrl":"https://example.com/file"}`)
	env := map[string]string{"ALIPAY_APP_ID": "env-app", "ALIPAY_PRIVATE_KEY": "env-private", "ALIPAY_PUBLIC_KEY": "env-public", "ALIPAY_PRODUCTION": "false", "ALIPAY_NOTIFY_URL": ""}
	lookup := func(key string) (string, bool) { value, exists := env[key]; return value, exists }
	cfg, ready, err := readAlipayConfig(path, true, lookup)
	if err != nil || !ready || cfg.appID != "env-app" || cfg.privateKey != "env-private" || cfg.publicKey != "env-public" || cfg.production || cfg.notifyURL != "" {
		t.Fatal("environment values did not override file configuration")
	}
	env["ALIPAY_PRIVATE_KEY"] = ""
	if _, ready, err := readAlipayConfig(path, true, lookup); err != nil || ready {
		t.Fatal("explicit empty credential should disable payment")
	}
}

func TestAlipayFileConfigEmptyCredentialsDisablePayment(t *testing.T) {
	path := writeAlipayTestConfig(t, `{"appId":"","privateKey":"","publicKey":"","production":false,"notifyUrl":""}`)
	if _, ready, err := readAlipayConfig(path, true, noAlipayEnv); err != nil || ready {
		t.Fatal("empty local configuration must keep payment disabled")
	}
	if _, ready, err := readAlipayConfig("", false, noAlipayEnv); err != nil || ready {
		t.Fatal("absent default file should allow disabled payment")
	}
}

func TestAlipayInvalidConfigDoesNotExposeContents(t *testing.T) {
	for _, body := range []string{
		`{"privateKey":"sensitive-test-value",`,
		`{"unexpected-sensitive-test-value":"secret"}`,
		`{"privateKey":123,"appId":"sensitive-test-value"}`,
		`{"production":"sensitive-test-value"}`,
		`{"appId":"sensitive-test-value"} {}`,
		`null`,
		``,
	} {
		path := writeAlipayTestConfig(t, body)
		_, ready, err := readAlipayConfig(path, true, noAlipayEnv)
		if err == nil || ready {
			t.Fatal("invalid config was accepted")
		}
		if strings.Contains(err.Error(), "sensitive-test-value") || strings.Contains(err.Error(), "secret") {
			t.Fatal("error exposed config contents")
		}
	}
}

func TestAlipayConfigDiscoveryAndMissingExplicitPath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("ALIPAY_CONFIG_FILE", "")
	for _, key := range []string{"ALIPAY_APP_ID", "ALIPAY_PRIVATE_KEY", "ALIPAY_PUBLIC_KEY", "ALIPAY_NOTIFY_URL", "ALIPAY_PRODUCTION"} {
		t.Setenv(key, "")
	}
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config", "alipay.local.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	discovered, found, err := alipayConfigFile()
	if err != nil || !found || discovered != filepath.Join("config", "alipay.local.json") {
		t.Fatal("default project configuration was not found")
	}
	if payer, err := newAlipayPayer(); err != nil || payer != nil {
		t.Fatal("blank discovered file should disable payment")
	}
	t.Setenv("ALIPAY_CONFIG_FILE", filepath.Join(root, "missing.json"))
	if payer, err := newAlipayPayer(); err == nil || payer != nil {
		t.Fatal("missing explicit configuration should report an error")
	}
}

func TestAlipayExplicitConfigPathLoadsFile(t *testing.T) {
	path := writeAlipayTestConfig(t, `{"appId":"test-file-app","privateKey":"test-file-private","publicKey":"test-file-public"}`)
	t.Setenv("ALIPAY_CONFIG_FILE", path)
	// Unset the inherited credential variables to exercise actual file loading.
	for _, key := range []string{"ALIPAY_APP_ID", "ALIPAY_PRIVATE_KEY", "ALIPAY_PUBLIC_KEY"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, ready, err := loadAlipayConfig()
	if err != nil || !ready || cfg.appID != "test-file-app" || cfg.privateKey != "test-file-private" || cfg.publicKey != "test-file-public" {
		t.Fatal("explicit file was not loaded by the payment config loader")
	}
}
