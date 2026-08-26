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
