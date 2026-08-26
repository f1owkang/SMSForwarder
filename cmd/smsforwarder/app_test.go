package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"smsforwarder/internal/channel"
)

type fakeCh struct {
	name  string
	err   error
	calls int
}

func (f *fakeCh) Name() string { return f.name }
func (f *fakeCh) Send(ctx context.Context, m channel.Message) error {
	f.calls++
	return f.err
}

func newTestApp(chs ...channel.Channel) (*App, *[]string, *[]string) {
	var warns, infos []string
	app := &App{
		recipients: []recipientRuntime{{Name: "r1", Chans: chs}},
		keywordOf:  func(s string) string { return "关键词A" },
		warn:       func(msg string) { warns = append(warns, msg) },
		info:       func(msg string) { infos = append(infos, msg) },
		record:     func(fields map[string]any, summary string) {},
	}
	return app, &warns, &infos
}

func TestChainStopsOnFirstSuccess(t *testing.T) {
	okCh := &fakeCh{name: "a"}
	fail := &fakeCh{name: "b", err: errors.New("boom")}
	after := &fakeCh{name: "c"}
	app, warns, _ := newTestApp(fail, okCh, after)
	del := app.HandleSMS(channel.Message{Number: "10086", Text: "t", Timestamp: "now"})
	if !del {
		t.Fatal("任一成功即视为成功")
	}
	if fail.calls != 1 || okCh.calls != 1 || after.calls != 0 {
		t.Fatalf("链路应在首个成功处停止: %d %d %d", fail.calls, okCh.calls, after.calls)
	}
	joined := strings.Join(*warns, "\n")
	if !strings.Contains(joined, "boom") {
		t.Fatalf("失败渠道应告警: %v", joined)
	}
}

func TestAllChannelsFailed(t *testing.T) {
	f1 := &fakeCh{name: "a", err: errors.New("x")}
	f2 := &fakeCh{name: "b", err: errors.New("y")}
	app, _, _ := newTestApp(f1, f2)
	if app.HandleSMS(channel.Message{Number: "n", Text: "t"}) {
		t.Fatal("全部失败应返回 false")
	}
}

type panicCh struct{}

func (p panicCh) Name() string                                      { return "panic" }
func (p panicCh) Send(ctx context.Context, m channel.Message) error { panic("kaboom") }

func TestPanicIsolated(t *testing.T) {
	app, _, _ := newTestApp(panicCh{})
	if app.HandleSMS(channel.Message{Number: "n", Text: "t"}) {
		t.Fatal("panic 渠道不应算成功")
	}
}

func TestPanicDoesNotBlockLaterRecipients(t *testing.T) {
	healthy := &fakeCh{name: "ok"}
	app := &App{
		recipients: []recipientRuntime{
			{Name: "r1", Chans: []channel.Channel{panicCh{}}},
			{Name: "r2", Chans: []channel.Channel{healthy}},
		},
		keywordOf: func(s string) string { return "关键词A" },
		warn:      func(string) {},
		info:      func(string) {},
		record:    func(map[string]any, string) {},
	}
	if !app.HandleSMS(channel.Message{Number: "n", Text: "t"}) {
		t.Fatal("r2 健康渠道应使整体视为已送达")
	}
	if healthy.calls != 1 {
		t.Fatalf("r2 渠道应恰好被调用一次: %d", healthy.calls)
	}
}

func TestRecordEmittedOnceOnAllFailure(t *testing.T) {
	var calls int
	var fields map[string]any
	app := &App{
		recipients: []recipientRuntime{{Name: "r1", Chans: []channel.Channel{
			&fakeCh{name: "a", err: errors.New("x")},
			panicCh{},
		}}},
		keywordOf: func(string) string { return "" },
		warn:      func(string) {},
		info:      func(string) {},
		record:    func(f map[string]any, _ string) { calls++; fields = f },
	}
	if app.HandleSMS(channel.Message{Number: "10086", Text: "t"}) {
		t.Fatal("全部失败或 panic 应返回 false")
	}
	if calls != 1 {
		t.Fatalf("Record 应恰好调用一次: %d", calls)
	}
	if fields["status"] != "failed" {
		t.Fatalf("全失败状态应为 failed: %v", fields["status"])
	}
	fwd, ok := fields["forwarded_to"].([]string)
	if !ok || len(fwd) != 0 {
		t.Fatalf("forwarded_to 应为空切片而非 null: %#v", fields["forwarded_to"])
	}
}

type captureCh struct{ dst *channel.Message }

func (c captureCh) Name() string                                      { return "cap" }
func (c captureCh) Send(ctx context.Context, m channel.Message) error { *c.dst = m; return nil }

func TestKeywordInjectedIntoMessage(t *testing.T) {
	var seen channel.Message
	app, _, _ := newTestApp(captureCh{&seen})
	app.HandleSMS(channel.Message{Number: "n", Text: "body"})
	if seen.Keyword != "关键词A" {
		t.Fatalf("Keyword 未注入: %+v", seen)
	}
}
