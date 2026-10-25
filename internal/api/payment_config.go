package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type alipayConfig struct {
	appID      string
	privateKey string
	publicKey  string
	production bool
	notifyURL  string
}

type alipayFileConfig struct {
	AppID      string `json:"appId"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Production bool   `json:"production"`
	NotifyURL  string `json:"notifyUrl"`
}

// Default configuration is local to the project or executable and is never embedded
// in the binary. An explicit path must exist so deployment mistakes are visible.
func alipayConfigFile() (string, bool, error) {
	if path := strings.TrimSpace(os.Getenv("ALIPAY_CONFIG_FILE")); path != "" {
		return path, true, nil
	}
	candidates := []string{filepath.Join("config", "alipay.local.json")}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "config", "alipay.local.json"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("无法访问支付宝配置文件 %s", path)
		}
	}
	return "", false, nil
}

func loadAlipayConfig() (alipayConfig, bool, error) {
	path, required, err := alipayConfigFile()
	if err != nil {
		return alipayConfig{}, false, err
	}
	return readAlipayConfig(path, required, os.LookupEnv)
}

func readAlipayConfig(path string, required bool, lookup func(string) (string, bool)) (alipayConfig, bool, error) {
	var file alipayFileConfig
	if required {
		data, err := os.ReadFile(path)
		if err != nil {
			return alipayConfig{}, false, fmt.Errorf("无法读取支付宝配置文件 %s", path)
		}
		// Windows editors may write a UTF-8 BOM. Do not include parser errors or
		// config contents in returned errors, since they may contain key material.
		data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if len(data) == 0 || data[0] != '{' || decoder.Decode(&file) != nil {
			return alipayConfig{}, false, fmt.Errorf("支付宝配置文件 %s 格式错误，请检查 JSON 字段和类型", path)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return alipayConfig{}, false, fmt.Errorf("支付宝配置文件 %s 只能包含一个 JSON 对象", path)
		}
	}
	cfg := alipayConfig{
		appID: file.AppID, privateKey: file.PrivateKey, publicKey: file.PublicKey,
		production: file.Production, notifyURL: file.NotifyURL,
	}
	for _, field := range []struct {
		name  string
		value *string
	}{
		{"ALIPAY_APP_ID", &cfg.appID}, {"ALIPAY_PRIVATE_KEY", &cfg.privateKey},
		{"ALIPAY_PUBLIC_KEY", &cfg.publicKey}, {"ALIPAY_NOTIFY_URL", &cfg.notifyURL},
	} {
		if value, exists := lookup(field.name); exists {
			*field.value = value
		}
		*field.value = strings.TrimSpace(*field.value)
	}
	if value, exists := lookup("ALIPAY_PRODUCTION"); exists {
		cfg.production = value == "true"
	}
	if cfg.appID == "" || cfg.privateKey == "" || cfg.publicKey == "" {
		return alipayConfig{}, false, nil
	}
	return cfg, true, nil
}
