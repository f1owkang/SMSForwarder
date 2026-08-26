package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validYAML = `
log_file: /var/log/s.jsonl
auto_delete: true
heartbeat: 6h
recipients:
  - name: main1
    channels:
      - type: pushplus
        token: tok1
      - type: sms
        phone: "18100000000"
  - name: aux
    channels:
      - type: bark
        device_key: bk
      - type: serverchan
        send_key: SCTx
      - type: telegram
        bot_token: "1:A"
        chat_id: "42"
      - type: webhook
        url: https://example.com/hook
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("合法配置解析失败: %v", err)
	}
	if !cfg.AutoDelete || cfg.LogFile != "/var/log/s.jsonl" {
		t.Fatalf("标量字段解析错误: %+v", cfg)
	}
	if cfg.Heartbeat.Duration != 6*time.Hour {
		t.Fatalf("heartbeat 解析错误: %v", cfg.Heartbeat.Duration)
	}
	if len(cfg.Recipients) != 2 || len(cfg.Recipients[1].Channels) != 4 {
		t.Fatalf("recipients 解析错误: %+v", cfg.Recipients)
	}
}

func TestHeartbeatDefault24hAndZeroOff(t *testing.T) {
	noHB := strings.Replace(validYAML, "heartbeat: 6h\n", "", 1)
	cfg, err := Load(writeTemp(t, noHB))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Heartbeat == nil || cfg.Heartbeat.Duration != 24*time.Hour {
		t.Fatalf("缺省 heartbeat 应为 24h: %+v", cfg.Heartbeat)
	}
	zero := strings.Replace(validYAML, "heartbeat: 6h", "heartbeat: 0", 1)
	cfg, err = Load(writeTemp(t, zero))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Heartbeat.Duration != 0 {
		t.Fatalf("heartbeat: 0 应表示关闭: %v", cfg.Heartbeat.Duration)
	}
}

func TestEmptyRecipientsRejected(t *testing.T) {
	_, err := Load(writeTemp(t, "log_file: \"\"\n"))
	if err == nil || !strings.Contains(err.Error(), "recipients") {
		t.Fatalf("应报 recipients 为空: %v", err)
	}
}

func TestChannelFieldErrors(t *testing.T) {
	cases := []struct{ yml, wantSub string }{
		{"recipients:\n  - name: a\n    channels:\n      - type: pushplus\n", "token"},
		{"recipients:\n  - name: a\n    channels:\n      - type: bark\n        device_key: \"\"\n", "device_key"},
		{"recipients:\n  - name: a\n    channels:\n      - type: telegram\n        bot_token: t\n", "chat_id"},
		{"recipients:\n  - name: a\n    channels:\n      - type: webhook\n        url: ftp://x\n", "url"},
		{"recipients:\n  - name: a\n    channels:\n      - type: sms\n        phone: abc\n", "phone"},
		{"recipients:\n  - name: a\n    channels:\n      - type: carrier_pigeon\n", "carrier_pigeon"},
		{"recipients:\n  - name: a\n    channels: []\n", "channels 不能为空"},
	}
	for _, c := range cases {
		_, err := Load(writeTemp(t, c.yml))
		if err == nil || !strings.Contains(err.Error(), c.wantSub) || !strings.Contains(err.Error(), `"a"`) {
			t.Fatalf("用例 %q 应报错并含接收者名: %v", c.wantSub, err)
		}
	}
}

func TestDuplicateRecipientName(t *testing.T) {
	yml := strings.Replace(validYAML, "name: aux", "name: main1", 1)
	_, err := Load(writeTemp(t, yml))
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重名应报错: %v", err)
	}
}

func TestResolvePathFlagWins(t *testing.T) {
	p := writeTemp(t, validYAML)
	got, err := ResolvePath(p)
	if err != nil || got != p {
		t.Fatalf("-c 指定路径应直接采用: %v %v", got, err)
	}
	_, err = ResolvePath(filepath.Join(t.TempDir(), "nope.yml"))
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("指定路径缺失应报错: %v", err)
	}
}

func TestResolvePathFallback(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	if _, err := ResolvePath(""); err == nil {
		t.Fatal("无任何候选文件时应报错")
	}
	if err := os.WriteFile("./config.yml", []byte(validYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePath("")
	if err != nil || got != "config.yml" {
		t.Fatalf("应回退到 ./config.yml: %v %v", got, err)
	}
}

func TestFindWordlistOrder(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	os.WriteFile(cfgPath, []byte(validYAML), 0o644)
	if got := FindWordlist(cfgPath, "stopwords.txt"); got != "" {
		t.Fatalf("词库缺失应返回空串: %q", got)
	}
	os.WriteFile(filepath.Join(dir, "userwords.txt"), []byte("测试词 10 n\n"), 0o644)
	got := FindWordlist(cfgPath, "userwords.txt")
	if got != filepath.Join(dir, "userwords.txt") {
		t.Fatalf("优先配置目录: %q", got)
	}
}
