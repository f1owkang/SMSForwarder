package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, errors.New("disk full") }

func newTestLogger() (*Logger, *bytes.Buffer, *bytes.Buffer) {
	con := &bytes.Buffer{}
	f := &bytes.Buffer{}
	lg := New(con, f)
	lg.now = func() time.Time { return time.Date(2026, 8, 26, 12, 34, 56, 0, time.Local) }
	return lg, con, f
}

func TestInfoWritesBothSinks(t *testing.T) {
	lg, con, f := newTestLogger()
	lg.Info("短信监听服务已启动")
	if got := con.String(); got != "[INFO] 短信监听服务已启动\n" {
		t.Fatalf("控制台格式错误: %q", got)
	}
	var rec map[string]string
	if err := json.Unmarshal(f.Bytes(), &rec); err != nil {
		t.Fatalf("文件不是合法 JSONL: %v", err)
	}
	want := map[string]string{
		"type":      "log",
		"timestamp": "2026-08-26 12:34:56",
		"level":     "INFO",
		"message":   "短信监听服务已启动",
	}
	for k, v := range want {
		if rec[k] != v {
			t.Fatalf("字段 %s = %q, 期望 %q", k, rec[k], v)
		}
	}
}

func TestWarnErrorLevels(t *testing.T) {
	lg, con, _ := newTestLogger()
	lg.Warn("警告信息")
	lg.Error("错误信息")
	lines := strings.Split(strings.TrimSpace(con.String()), "\n")
	if lines[0] != "[WARNING] 警告信息" || lines[1] != "[ERROR] 错误信息" {
		t.Fatalf("级别前缀错误: %v", lines)
	}
}

func TestNilFileSkipsFileSink(t *testing.T) {
	con := &bytes.Buffer{}
	lg := New(con, nil)
	lg.Info("hello")
	if con.String() != "[INFO] hello\n" {
		t.Fatalf("控制台输出错误: %q", con.String())
	}
}

func TestRecordFlatFields(t *testing.T) {
	lg, con, f := newTestLogger()
	lg.Record(map[string]any{
		"number": "10086", "text": "hi", "timestamp": "2026-08-26 12:00:00",
		"forwarded_to": []string{"main1 (pushplus)"}, "status": "ok",
	}, "[√] 已转发")
	var rec map[string]any
	if err := json.Unmarshal(f.Bytes(), &rec); err != nil {
		t.Fatalf("Record 不是合法 JSONL: %v", err)
	}
	if rec["status"] != "ok" || rec["type"] != "record" {
		t.Fatalf("status/type 字段错误: %v", rec)
	}
	if con.String() != "[INFO] [√] 已转发\n" {
		t.Fatalf("Record 控制台摘要错误: %q", con.String())
	}
}

func TestLogWriteFailureFallsToStderr(t *testing.T) {
	var errSink bytes.Buffer
	lg := New(&bytes.Buffer{}, failingWriter{})
	lg.stderr = &errSink
	lg.Info("重要消息")
	out := errSink.String()
	if !strings.Contains(out, "disk full") || !strings.Contains(out, "重要消息") {
		t.Fatalf("写文件失败应落 stderr 且带原消息: %q", out)
	}
}

func TestRecordWriteFailureFallsToStderr(t *testing.T) {
	var errSink bytes.Buffer
	lg := New(&bytes.Buffer{}, failingWriter{})
	lg.stderr = &errSink
	lg.Record(map[string]any{"number": "10086"}, "[√] 已转发")
	out := errSink.String()
	if !strings.Contains(out, "disk full") || !strings.Contains(out, "已转发") {
		t.Fatalf("Record 写文件失败应落 stderr: %q", out)
	}
}

func TestWriteOKNoStderr(t *testing.T) {
	var errSink bytes.Buffer
	lg := New(&bytes.Buffer{}, &bytes.Buffer{})
	lg.stderr = &errSink
	lg.Info("正常")
	lg.Record(map[string]any{"n": "1"}, "[√] 正常")
	if errSink.Len() != 0 {
		t.Fatalf("写文件成功不应落 stderr: %q", errSink.String())
	}
}
