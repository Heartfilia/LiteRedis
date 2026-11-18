package config

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	// ThirdPartyPlatformARDM is the AnotherRedisDesktopManager export format (.ano).
	ThirdPartyPlatformARDM = "ardm"
	// ThirdPartyPlatformTinyRDM is the Tiny RDM export format (.zip with connections.yaml).
	ThirdPartyPlatformTinyRDM = "tinyrdm"

	maxTinyRDMYAMLSize = 8 << 20
)

// ThirdPartySkippedItem explains why one source connection was not imported.
// Reasons stay in Chinese to match the existing import summary messages.
type ThirdPartySkippedItem struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ThirdPartyParseResult is the platform-independent result of reading an export.
type ThirdPartyParseResult struct {
	Connections []ConnectionConfig
	Skipped     []ThirdPartySkippedItem
	Warnings    []string
	Total       int
}

// ThirdPartyImportPreview is shown to the user before anything is written.
// Token is a short-lived, single-use handle for the in-memory parsed result.
type ThirdPartyImportPreview struct {
	Platform     string                  `json:"platform"`
	FileName     string                  `json:"file_name"`
	Canceled     bool                    `json:"canceled,omitempty"`
	Total        int                     `json:"total"`
	Importable   int                     `json:"importable"`
	Skipped      int                     `json:"skipped"`
	Renamed      int                     `json:"renamed,omitempty"`
	SkippedItems []ThirdPartySkippedItem `json:"skipped_items,omitempty"`
	Warnings     []string                `json:"warnings,omitempty"`
	Token        string                  `json:"token,omitempty"`
}

// ParseThirdPartyExport decodes an export file without touching the local store.
func ParseThirdPartyExport(platform string, data []byte) (ThirdPartyParseResult, error) {
	switch platform {
	case ThirdPartyPlatformARDM:
		return parseARDMExport(data)
	case ThirdPartyPlatformTinyRDM:
		return parseTinyRDMExport(data)
	default:
		return ThirdPartyParseResult{}, fmt.Errorf("不支持的第三方平台: %s", platform)
	}
}

// CountThirdPartyNameConflicts reports how many incoming connections would be
// renamed because the same group already has that name (or the file repeats it).
func CountThirdPartyNameConflicts(in []ConnectionConfig) int {
	storeMu.RLock()
	defer storeMu.RUnlock()

	store, err := loadStore()
	if err != nil {
		return 0
	}
	used := make(map[string]struct{}, len(store.Connections)+len(in))
	for _, cfg := range store.Connections {
		used[connectionNameKey(cfg.Group, cfg.Name)] = struct{}{}
	}
	conflicts := 0
	for _, cfg := range in {
		key := connectionNameKey(cfg.Group, cfg.Name)
		if _, exists := used[key]; exists {
			conflicts++
		}
		used[key] = struct{}{}
	}
	return conflicts
}

func connectionNameKey(group, name string) string {
	return strings.ToLower(strings.TrimSpace(group)) + "\x00" + strings.ToLower(strings.TrimSpace(name))
}

// FilterThirdPartyDuplicates removes connections whose Redis endpoint already
// exists locally or repeats earlier in the same file. Existing connections are
// never modified; duplicates are returned as skip reasons instead.
//
// The endpoint is host:port, plus the SSH jump host when a tunnel is used, so
// the same Redis host reached through different SSH servers remains distinct.
func FilterThirdPartyDuplicates(in []ConnectionConfig) ([]ConnectionConfig, []ThirdPartySkippedItem) {
	storeMu.RLock()
	defer storeMu.RUnlock()

	store, err := loadStore()
	if err != nil {
		// Do not drop data when the local store cannot be read; applying the
		// import will surface the real configuration error.
		return in, nil
	}

	existing := make(map[string]string, len(store.Connections))
	for _, cfg := range store.Connections {
		existing[connectionEndpointKey(cfg)] = strings.TrimSpace(cfg.Name)
	}

	kept := make([]ConnectionConfig, 0, len(in))
	skipped := make([]ThirdPartySkippedItem, 0)
	seen := make(map[string]string, len(in))
	for _, cfg := range in {
		key := connectionEndpointKey(cfg)
		label := connectionEndpointLabel(cfg)
		if name, ok := existing[key]; ok {
			if name == "" {
				name = "未命名连接"
			}
			skipped = append(skipped, ThirdPartySkippedItem{
				Name:   strings.TrimSpace(cfg.Name),
				Reason: "地址已存在（" + label + "），不会覆盖现有连接「" + name + "」",
			})
			continue
		}
		if name, ok := seen[key]; ok {
			skipped = append(skipped, ThirdPartySkippedItem{
				Name:   strings.TrimSpace(cfg.Name),
				Reason: "与本次导入的「" + name + "」地址相同（" + label + "），只保留第一条",
			})
			continue
		}
		seen[key] = strings.TrimSpace(cfg.Name)
		kept = append(kept, cfg)
	}
	return kept, skipped
}

func connectionEndpointKey(cfg ConnectionConfig) string {
	host, port := connectionEndpointHostPort(cfg)
	endpoint := net.JoinHostPort(host, strconv.Itoa(port))
	if cfg.SSHEnabled && cfg.SSH != nil {
		sshHost := normalizeHost(cfg.SSH.Host)
		if strings.TrimSpace(cfg.SSH.Host) != "" {
			sshPort := cfg.SSH.Port
			if sshPort < 1 || sshPort > 65535 {
				sshPort = 22
			}
			return "ssh://" + strings.ToLower(strings.TrimSpace(cfg.SSH.User)) + "@" +
				net.JoinHostPort(sshHost, strconv.Itoa(sshPort)) + "/" + endpoint
		}
	}
	return endpoint
}

func connectionEndpointLabel(cfg ConnectionConfig) string {
	host, port := connectionEndpointHostPort(cfg)
	label := net.JoinHostPort(host, strconv.Itoa(port))
	if cfg.SSHEnabled && cfg.SSH != nil && strings.TrimSpace(cfg.SSH.Host) != "" {
		label += " · SSH " + strings.TrimSpace(cfg.SSH.Host)
	}
	return label
}

// connectionEndpointHostPort resolves the address used for duplicate checks.
// Cluster connections may omit the single host field and only carry node
// addresses, so fall back to the first node instead of treating them as
// 127.0.0.1.
func connectionEndpointHostPort(cfg ConnectionConfig) (string, int) {
	host := strings.TrimSpace(cfg.Host)
	port := cfg.Port
	if host == "" && cfg.IsCluster {
		for _, addr := range cfg.ClusterAddrs {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			if h, p, err := net.SplitHostPort(addr); err == nil {
				if parsed, err := strconv.Atoi(p); err == nil && parsed >= 1 && parsed <= 65535 {
					return normalizeHost(h), parsed
				}
				return normalizeHost(h), 6379
			}
		}
	}
	if port < 1 || port > 65535 {
		port = 6379
	}
	return normalizeHost(host), port
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		host = "127.0.0.1"
	}
	return host
}

// --- AnotherRedisDesktopManager (.ano) ---

type ardmBundle struct {
	Connections []ardmConnection `json:"connections"`
	Groups      []ardmGroup      `json:"groups"`
}

type ardmGroup struct {
	ID   flexString `json:"id"`
	Name string     `json:"name"`
}

type ardmConnection struct {
	Name               string              `json:"name"`
	Host               string              `json:"host"`
	Port               flexInt             `json:"port"`
	Auth               string              `json:"auth"`
	Username           string              `json:"username"`
	DB                 flexInt             `json:"db"`
	Cluster            bool                `json:"cluster"`
	ConnectionReadOnly bool                `json:"connectionReadOnly"`
	GroupID            flexString          `json:"groupId"`
	Group              string              `json:"group"`
	SSLOptions         *ardmSSLOptions     `json:"sslOptions"`
	SSHOptions         *ardmSSHOptions     `json:"sshOptions"`
	SentinelOptions    *ardmSentinelOption `json:"sentinelOptions"`
}

type ardmSSLOptions struct {
	Key           string `json:"key"`
	Cert          string `json:"cert"`
	CA            string `json:"ca"`
	ServerName    string `json:"servername"`
	ServerNameAlt string `json:"serverName"`
}

type ardmSSHOptions struct {
	Host       string  `json:"host"`
	Port       flexInt `json:"port"`
	Username   string  `json:"username"`
	Password   string  `json:"password"`
	PrivateKey string  `json:"privatekey"`
	Passphrase string  `json:"passphrase"`
}

type ardmSentinelOption struct {
	MasterName   string `json:"masterName"`
	NodePassword string `json:"nodePassword"`
}

func parseARDMExport(data []byte) (ThirdPartyParseResult, error) {
	payload, err := decodeARDMPayload(data)
	if err != nil {
		return ThirdPartyParseResult{}, err
	}

	var connections []ardmConnection
	var groups []ardmGroup
	switch {
	case len(payload) > 0 && payload[0] == '[':
		if err := json.Unmarshal(payload, &connections); err != nil {
			return ThirdPartyParseResult{}, fmt.Errorf("不是有效的 AnotherRedisDesktopManager 导出文件: %w", err)
		}
	default:
		var bundle ardmBundle
		if err := json.Unmarshal(payload, &bundle); err != nil {
			return ThirdPartyParseResult{}, fmt.Errorf("不是有效的 AnotherRedisDesktopManager 导出文件: %w", err)
		}
		connections, groups = bundle.Connections, bundle.Groups
	}

	groupNames := make(map[string]string, len(groups))
	for _, group := range groups {
		id := strings.TrimSpace(string(group.ID))
		name := strings.TrimSpace(group.Name)
		if id != "" && name != "" {
			groupNames[id] = name
		}
	}

	result := ThirdPartyParseResult{}
	hasTLS, hasDB := false, false
	for _, item := range connections {
		result.Total++

		name := strings.TrimSpace(item.Name)
		if name == "" {
			result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: "未命名连接", Reason: "缺少连接名称"})
			continue
		}
		host := strings.TrimSpace(item.Host)
		if host == "" {
			host = "127.0.0.1"
		}
		if item.SentinelOptions != nil {
			result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持 Sentinel 连接"})
			continue
		}
		ssl := item.SSLOptions
		if ssl != nil {
			if strings.TrimSpace(ssl.Key) != "" || strings.TrimSpace(ssl.Cert) != "" {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持客户端证书/私钥（双向 TLS）"})
				continue
			}
			sni := strings.TrimSpace(ssl.ServerName)
			if sni == "" {
				sni = strings.TrimSpace(ssl.ServerNameAlt)
			}
			if sni != "" && !strings.EqualFold(sni, host) {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持自定义 TLS SNI"})
				continue
			}
		}

		cfg := ConnectionConfig{
			Name:      name,
			Password:  item.Auth,
			Username:  item.Username,
			ReadOnly:  item.ConnectionReadOnly,
			Host:      host,
			Port:      int(item.Port),
			DB:        int(item.DB),
			IsCluster: item.Cluster,
		}
		if cfg.Port < 1 || cfg.Port > 65535 {
			cfg.Port = 6379
		}
		if cfg.DB < 0 {
			cfg.DB = 0
		}
		if cfg.DB > 0 {
			hasDB = true
		}
		if group := strings.TrimSpace(item.Group); group != "" {
			cfg.Group = group
		} else if name := groupNames[strings.TrimSpace(string(item.GroupID))]; name != "" {
			cfg.Group = name
		}
		if cfg.IsCluster {
			cfg.ClusterAddrs = []string{net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))}
		}

		if ssl != nil {
			cfg.TLSEnabled = true
			// ARDM always sets rejectUnauthorized: false. Keep the original
			// behavior, but say so in the preview instead of silently tightening it.
			cfg.TLSSkipVerify = true
			cfg.TLSCAFile = strings.TrimSpace(ssl.CA)
			hasTLS = true
			if cfg.TLSCAFile != "" && !fileExists(cfg.TLSCAFile) {
				result.Warnings = appendUnique(result.Warnings, fmt.Sprintf("「%s」的 TLS CA 文件不存在：%s", cfg.Name, cfg.TLSCAFile))
			}
		}

		if item.SSHOptions != nil {
			ssh := SSHConfig{
				Host:           strings.TrimSpace(item.SSHOptions.Host),
				Port:           int(item.SSHOptions.Port),
				User:           strings.TrimSpace(item.SSHOptions.Username),
				Password:       item.SSHOptions.Password,
				PrivateKeyPath: strings.TrimSpace(item.SSHOptions.PrivateKey),
				Passphrase:     item.SSHOptions.Passphrase,
			}
			if ssh.Port < 1 || ssh.Port > 65535 {
				ssh.Port = 22
			}
			if ssh.Host == "" || ssh.User == "" || (ssh.Password == "" && ssh.PrivateKeyPath == "") {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "SSH 配置不完整（缺少主机、用户名、密码或私钥）"})
				continue
			}
			if ssh.PrivateKeyPath != "" && !fileExists(ssh.PrivateKeyPath) {
				result.Warnings = appendUnique(result.Warnings, fmt.Sprintf("「%s」的 SSH 私钥文件不存在：%s", cfg.Name, ssh.PrivateKeyPath))
			}
			cfg.SSHEnabled = true
			cfg.SSH = &ssh
		}

		result.Connections = append(result.Connections, cfg)
	}

	if hasTLS {
		result.Warnings = appendUnique(result.Warnings, "AnotherRedisDesktopManager 的 TLS 连接默认不校验证书，导入后保持“跳过证书校验”。")
	}
	if hasDB {
		result.Warnings = appendUnique(result.Warnings, "已保留 ARDM 的数据库编号；它表示上次使用的库，不一定是业务默认库。")
	}
	return result, nil
}

func decodeARDMPayload(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}))
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("文件为空")
	}
	// Accept already-decoded JSON as well; users sometimes paste the content.
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return trimmed, nil
	}
	compact := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, string(trimmed))
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(compact)
		if err != nil {
			continue
		}
		decoded = bytes.TrimSpace(bytes.TrimPrefix(decoded, []byte{0xEF, 0xBB, 0xBF}))
		if len(decoded) > 0 && (decoded[0] == '{' || decoded[0] == '[') {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("不是有效的 AnotherRedisDesktopManager 导出文件（Base64 解码失败）")
}

// --- Tiny RDM (.zip / connections.yaml) ---

type tinyRDMConnection struct {
	Name        string              `yaml:"name"`
	Group       string              `yaml:"group"`
	LastDB      int                 `yaml:"last_db"`
	LastDBAlt   int                 `yaml:"lastDB"`
	Network     string              `yaml:"network"`
	Sock        string              `yaml:"sock"`
	Addr        string              `yaml:"addr"`
	Port        int                 `yaml:"port"`
	Username    string              `yaml:"username"`
	Password    string              `yaml:"password"`
	MarkColor   string              `yaml:"mark_color"`
	SSL         tinyRDMSSL          `yaml:"ssl"`
	SSH         tinyRDMSSH          `yaml:"ssh"`
	Sentinel    tinyRDMSentinel     `yaml:"sentinel"`
	Cluster     tinyRDMCluster      `yaml:"cluster"`
	Proxy       tinyRDMProxy        `yaml:"proxy"`
	Type        string              `yaml:"type"`
	Connections []tinyRDMConnection `yaml:"connections"`
}

type tinyRDMSSL struct {
	Enable        bool   `yaml:"enable"`
	KeyFile       string `yaml:"keyfile"`
	CertFile      string `yaml:"certfile"`
	CAFile        string `yaml:"cafile"`
	KeyFileAlt    string `yaml:"key_file"`
	CertFileAlt   string `yaml:"cert_file"`
	CAFileAlt     string `yaml:"ca_file"`
	AllowInsecure bool   `yaml:"allow_insecure"`
	SNI           string `yaml:"sni"`
}

func (ssl tinyRDMSSL) keyFile() string {
	if strings.TrimSpace(ssl.KeyFile) != "" {
		return strings.TrimSpace(ssl.KeyFile)
	}
	return strings.TrimSpace(ssl.KeyFileAlt)
}

func (ssl tinyRDMSSL) certFile() string {
	if strings.TrimSpace(ssl.CertFile) != "" {
		return strings.TrimSpace(ssl.CertFile)
	}
	return strings.TrimSpace(ssl.CertFileAlt)
}

func (ssl tinyRDMSSL) caFile() string {
	if strings.TrimSpace(ssl.CAFile) != "" {
		return strings.TrimSpace(ssl.CAFile)
	}
	return strings.TrimSpace(ssl.CAFileAlt)
}

type tinyRDMSSH struct {
	Enable     bool   `yaml:"enable"`
	Addr       string `yaml:"addr"`
	Port       int    `yaml:"port"`
	LoginType  string `yaml:"login_type"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	PKFile     string `yaml:"pk_file"`
	Passphrase string `yaml:"passphrase"`
}

type tinyRDMSentinel struct {
	Enable   bool   `yaml:"enable"`
	Master   string `yaml:"master"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type tinyRDMCluster struct {
	Enable bool `yaml:"enable"`
}

type tinyRDMProxy struct {
	Type     int    `yaml:"type"`
	Schema   string `yaml:"schema"`
	Addr     string `yaml:"addr"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func parseTinyRDMExport(data []byte) (ThirdPartyParseResult, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ThirdPartyParseResult{}, fmt.Errorf("不是有效的 Tiny RDM 导出压缩包: %w", err)
	}
	var target *zip.File
	for _, file := range reader.File {
		if strings.EqualFold(path.Base(file.Name), "connections.yaml") {
			target = file
			break
		}
	}
	if target == nil {
		return ThirdPartyParseResult{}, fmt.Errorf("压缩包里找不到 connections.yaml，可能不是 Tiny RDM 的导出文件")
	}
	if target.UncompressedSize64 > maxTinyRDMYAMLSize {
		return ThirdPartyParseResult{}, fmt.Errorf("connections.yaml 过大（超过 8MiB）")
	}
	stream, err := target.Open()
	if err != nil {
		return ThirdPartyParseResult{}, fmt.Errorf("读取 connections.yaml 失败: %w", err)
	}
	defer stream.Close()
	content, err := io.ReadAll(io.LimitReader(stream, maxTinyRDMYAMLSize+1))
	if err != nil {
		return ThirdPartyParseResult{}, fmt.Errorf("读取 connections.yaml 失败: %w", err)
	}
	if len(content) > maxTinyRDMYAMLSize {
		return ThirdPartyParseResult{}, fmt.Errorf("connections.yaml 过大（超过 8MiB）")
	}

	var connections []tinyRDMConnection
	if err := yaml.Unmarshal(content, &connections); err != nil {
		var wrapper struct {
			Connections []tinyRDMConnection `yaml:"connections"`
		}
		if err2 := yaml.Unmarshal(content, &wrapper); err2 != nil {
			return ThirdPartyParseResult{}, fmt.Errorf("connections.yaml 不是有效的 YAML: %w", err)
		}
		connections = wrapper.Connections
	}

	result := ThirdPartyParseResult{}
	walkTinyRDMConnections(connections, "", &result)
	hasDB := false
	for _, cfg := range result.Connections {
		if cfg.DB > 0 {
			hasDB = true
			break
		}
	}
	if hasDB {
		result.Warnings = appendUnique(result.Warnings, "Tiny RDM 的“上次使用数据库”会作为连接的默认 DB 导入，可在连接编辑里再确认。")
	}
	return result, nil
}

func walkTinyRDMConnections(items []tinyRDMConnection, group string, result *ThirdPartyParseResult) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Type), "group") {
			next := strings.TrimSpace(item.Name)
			if next == "" {
				next = group
			}
			walkTinyRDMConnections(item.Connections, next, result)
			continue
		}
		result.Total++

		name := strings.TrimSpace(item.Name)
		if name == "" {
			result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: "未命名连接", Reason: "缺少连接名称"})
			continue
		}
		host := strings.TrimSpace(item.Addr)
		if host == "" {
			host = "127.0.0.1"
		}
		if item.Sentinel.Enable {
			result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持 Sentinel 连接"})
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Network), "unix") {
			result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持 Unix Socket 连接"})
			continue
		}
		if item.SSL.Enable {
			if item.SSL.certFile() != "" || item.SSL.keyFile() != "" {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持客户端证书/私钥（双向 TLS）"})
				continue
			}
			if strings.TrimSpace(item.SSL.SNI) != "" && !strings.EqualFold(strings.TrimSpace(item.SSL.SNI), host) {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持自定义 TLS SNI"})
				continue
			}
		}

		cfg := ConnectionConfig{
			Name:      name,
			Username:  item.Username,
			Password:  item.Password,
			Host:      host,
			Port:      item.Port,
			DB:        item.LastDB,
			ReadOnly:  false,
			IconColor: strings.TrimSpace(item.MarkColor),
		}
		if cfg.DB == 0 {
			cfg.DB = item.LastDBAlt
		}
		if cfg.DB < 0 {
			cfg.DB = 0
		}
		if cfg.Port < 1 || cfg.Port > 65535 {
			cfg.Port = 6379
		}
		if g := strings.TrimSpace(item.Group); g != "" {
			cfg.Group = g
		} else {
			cfg.Group = group
		}

		if item.Cluster.Enable {
			cfg.IsCluster = true
			cfg.ClusterAddrs = []string{net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))}
		}
		if item.SSL.Enable {
			cfg.TLSEnabled = true
			cfg.TLSSkipVerify = item.SSL.AllowInsecure
			cfg.TLSCAFile = item.SSL.caFile()
			if cfg.TLSCAFile != "" && !fileExists(cfg.TLSCAFile) {
				result.Warnings = appendUnique(result.Warnings, fmt.Sprintf("「%s」的 TLS CA 文件不存在：%s", cfg.Name, cfg.TLSCAFile))
			}
		}

		if item.SSH.Enable {
			loginType := strings.ToLower(strings.TrimSpace(item.SSH.LoginType))
			if loginType == "agent" {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持 SSH Agent 登录"})
				continue
			}
			ssh := SSHConfig{
				Host:           strings.TrimSpace(item.SSH.Addr),
				Port:           item.SSH.Port,
				User:           strings.TrimSpace(item.SSH.Username),
				Password:       item.SSH.Password,
				PrivateKeyPath: strings.TrimSpace(item.SSH.PKFile),
				Passphrase:     item.SSH.Passphrase,
			}
			// Tiny RDM lets the user choose one method explicitly; keeping both
			// credentials could change which one the server accepts.
			switch loginType {
			case "pwd":
				ssh.PrivateKeyPath = ""
				ssh.Passphrase = ""
			case "pkfile":
				ssh.Password = ""
			}
			if ssh.Port < 1 || ssh.Port > 65535 {
				ssh.Port = 22
			}
			if ssh.Host == "" || ssh.User == "" || (ssh.Password == "" && ssh.PrivateKeyPath == "") {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "SSH 配置不完整（缺少主机、用户名、密码或私钥）"})
				continue
			}
			if ssh.PrivateKeyPath != "" && !fileExists(ssh.PrivateKeyPath) {
				result.Warnings = appendUnique(result.Warnings, fmt.Sprintf("「%s」的 SSH 私钥文件不存在：%s", cfg.Name, ssh.PrivateKeyPath))
			}
			cfg.SSHEnabled = true
			cfg.SSH = &ssh
		}

		switch item.Proxy.Type {
		case 1:
			result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "暂不支持跟随系统代理"})
			continue
		case 2:
			proxyURL, ok := buildTinyRDMProxyURL(item.Proxy)
			if !ok {
				result.Skipped = append(result.Skipped, ThirdPartySkippedItem{Name: name, Reason: "代理配置不完整或协议不支持"})
				continue
			}
			cfg.ProxyEnabled = true
			cfg.ProxyURL = proxyURL
		}

		result.Connections = append(result.Connections, cfg)
	}
}

func buildTinyRDMProxyURL(proxy tinyRDMProxy) (string, bool) {
	schema := strings.ToLower(strings.TrimSpace(proxy.Schema))
	if schema == "" {
		schema = "http"
	}
	switch schema {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", false
	}
	addr := strings.TrimSpace(proxy.Addr)
	if addr == "" || proxy.Port < 1 || proxy.Port > 65535 {
		return "", false
	}
	target := url.URL{Scheme: schema, Host: net.JoinHostPort(addr, strconv.Itoa(proxy.Port))}
	if proxy.Username != "" || proxy.Password != "" {
		target.User = url.UserPassword(proxy.Username, proxy.Password)
	}
	return target.String(), true
}

// --- helpers ---

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

// flexInt accepts JSON numbers and numeric strings (ARDM stores the port as a
// string while the user is editing it). Invalid values become zero, which the
// parser then normalizes to the protocol default.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	*f = 0
	if raw == "" || raw == "null" {
		return nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return nil
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		if value, err := strconv.ParseFloat(text, 64); err == nil {
			*f = flexInt(int(value))
		}
		return nil
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		*f = flexInt(int(value))
	}
	return nil
}

// flexString accepts JSON strings or numbers (legacy group IDs were numeric).
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	*f = ""
	if raw == "" || raw == "null" {
		return nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return nil
		}
		*f = flexString(text)
		return nil
	}
	*f = flexString(strings.Trim(raw, `"`))
	return nil
}
