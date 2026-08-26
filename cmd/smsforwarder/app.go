package main

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

	"smsforwarder/internal/channel"
	"smsforwarder/internal/config"
	"smsforwarder/internal/extract"
	"smsforwarder/internal/logging"
	"smsforwarder/internal/modem"
)

type recipientRuntime struct {
	Name  string
	Chans []channel.Channel
}

type App struct {
	recipients []recipientRuntime
	keywordOf  func(string) string
	warn       func(string)
	info       func(string)
	record     func(fields map[string]any, summary string)
}

func NewApp(cfg *config.Config, lg *logging.Logger, ext *extract.Extractor, hc *channel.HTTPClient, conn *dbus.Conn) (*App, error) {
	modemPath, err := modem.FirstModemPath(conn)
	if err != nil {
		return nil, err
	}
	app := &App{
		keywordOf: ext.Extract,
		warn:      lg.Warn,
		info:      lg.Info,
		record:    lg.Record,
	}
	for _, r := range cfg.Recipients {
		rt := recipientRuntime{Name: r.Name}
		for _, cc := range r.Channels {
			ch, err := buildOne(cc, hc, conn, modemPath)
			if err != nil {
				return nil, err
			}
			rt.Chans = append(rt.Chans, ch)
		}
		app.recipients = append(app.recipients, rt)
	}
	return app, nil
}

func buildOne(cc config.ChannelConfig, hc *channel.HTTPClient, conn *dbus.Conn, modemPath dbus.ObjectPath) (channel.Channel, error) {
	switch cc.Type {
	case "pushplus":
		return channel.NewPushPlus(cc.Token, hc), nil
	case "serverchan":
		return channel.NewServerChan(cc.SendKey, hc), nil
	case "bark":
		return channel.NewBark(cc.Server, cc.DeviceKey, hc), nil
	case "telegram":
		return channel.NewTelegram(cc.BotToken, cc.ChatID, cc.Proxy, hc)
	case "webhook":
		return channel.NewWebhook(cc.URL, hc), nil
	case "sms":
		return channel.NewSMS(conn, modemPath), nil
	default:
		return nil, fmt.Errorf("未知渠道类型 %q", cc.Type)
	}
}

func (a *App) HandleSMS(m channel.Message) (delivered bool) {
	defer func() {
		if r := recover(); r != nil {
			a.warn(fmt.Sprintf("[!] 处理短信异常（已恢复）: %v", r))
			delivered = false
		}
	}()
	m.Keyword = a.keywordOf(m.Text)
	var sentTo []string
	for _, rc := range a.recipients {
		ok := false
		for _, ch := range rc.Chans {
			if err := ch.Send(context.Background(), m); err != nil {
				a.warn(fmt.Sprintf("[!] %s 渠道 %s 失败: %v", rc.Name, ch.Name(), err))
				continue
			}
			sentTo = append(sentTo, fmt.Sprintf("%s (%s)", rc.Name, ch.Name()))
			ok = true
			break
		}
		if !ok {
			a.warn(fmt.Sprintf("[!] 接收者 %s 所有渠道均失败", rc.Name))
		}
	}
	fields := map[string]any{
		"number":       m.Number,
		"text":         m.Text,
		"timestamp":    m.Timestamp,
		"forwarded_to": sentTo,
	}
	status := "failed"
	summary := fmt.Sprintf("[!] 转发失败: %s", m.Number)
	if len(sentTo) > 0 {
		status = "ok"
		summary = fmt.Sprintf("[√] 已转发: %s -> %v", m.Number, sentTo)
	}
	fields["status"] = status
	a.record(fields, summary)
	return len(sentTo) > 0
}

func (a *App) RunSelfTest() int {
	testMsg := channel.Message{
		Number:    "10086",
		Text:      "这是一条 SMSForwarder 测试消息",
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
	testMsg.Keyword = a.keywordOf(testMsg.Text)
	allOK := true
	for _, rc := range a.recipients {
		rcOK := false
		for _, ch := range rc.Chans {
			if err := ch.Send(context.Background(), testMsg); err != nil {
				a.info(fmt.Sprintf("[测试] %s 渠道 %s 失败: %v", rc.Name, ch.Name(), err))
				continue
			}
			a.info(fmt.Sprintf("[√] 测试消息送达 %s (%s)", rc.Name, ch.Name()))
			rcOK = true
		}
		if !rcOK {
			allOK = false
			a.warn(fmt.Sprintf("[!] 接收者 %s 所有渠道均失败", rc.Name))
		}
	}
	if allOK {
		return 0
	}
	return 1
}
