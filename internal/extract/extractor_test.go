package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWordlist(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	os.WriteFile(p, []byte(content), 0o644)
	return p
}

func TestVerificationCodeVariants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"您的验证码是884825，5分钟内有效", "验证码【884825】"},
		{"【某某银行】校验码 1234，切勿泄露", "验证码【1234】"},
		{"动态码:90210 请勿告知他人", "验证码【90210】"},
		{"验证码 884825523 多位数字不算", ""},
		{"验证码12345678 新验证码9988", "验证码【9988】"},
		{"随便聊天的普通短信内容", ""},
	}
	for _, c := range cases {
		if got := matchCode(c.in); got != c.want {
			t.Fatalf("matchCode(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

func TestKeywordFiltering(t *testing.T) {
	e := &Extractor{stopwords: map[string]bool{"通知": true, "您好": true}}
	e.extractFn = func(_ string, _ int) []string {
		return []string{"通知", "您", "12345", "订单", "发货", "物流", "提醒"}
	}
	got := e.Extract("任意文本")
	if got != "订单、发货、物流、提醒" {
		t.Fatalf("关键词过滤结果错误: %q", got)
	}
}

func TestExtractPrefersCodeOverKeywords(t *testing.T) {
	sw := writeWordlist(t, "stopwords.txt", "")
	user := writeWordlist(t, "userwords.txt", "登录验证 10 n\n")
	e, err := New(sw, user)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	if got := e.Extract("您的登录验证码是884825，5分钟内有效"); got != "验证码【884825】" {
		t.Fatalf("真实分词下验证码优先: %q", got)
	}
}

func TestMissingStopwordsWarnsButWorks(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "absent.txt"), "")
	if err != nil {
		t.Fatalf("停用词缺失不应报错: %v", err)
	}
	if len(e.Warnings()) == 0 || !strings.Contains(e.Warnings()[0], "停用词") {
		t.Fatalf("应有停用词缺失告警: %v", e.Warnings())
	}
	if got := e.Extract("您的验证码是6688"); got != "验证码【6688】" {
		t.Fatalf("降级仍要能提取验证码: %q", got)
	}
}
