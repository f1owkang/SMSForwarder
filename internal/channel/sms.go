package channel

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	modemDest      = "org.freedesktop.ModemManager1"
	ifaceMessaging = "org.freedesktop.ModemManager1.Modem.Messaging"
	ifaceSms       = "org.freedesktop.ModemManager1.Sms"
)

type SMS struct {
	conn      *dbus.Conn
	modemPath dbus.ObjectPath
}

func NewSMS(conn *dbus.Conn, modemPath dbus.ObjectPath) *SMS {
	return &SMS{conn: conn, modemPath: modemPath}
}

func (s *SMS) Name() string { return "sms" }

func smsCreateProps(number, text string) map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"number": dbus.MakeVariant(number),
		"text":   dbus.MakeVariant(text),
	}
}

func (s *SMS) Send(ctx context.Context, m Message) error {
	messaging := s.conn.Object(modemDest, s.modemPath)
	var smsPath dbus.ObjectPath
	err := messaging.CallWithContext(ctx, ifaceMessaging+".Create", 0,
		smsCreateProps(m.Number, m.Text)).Store(&smsPath)
	if err != nil {
		return fmt.Errorf("创建短信对象失败: %w", err)
	}
	err = s.conn.Object(modemDest, smsPath).CallWithContext(ctx, ifaceSms+".Send", 0).Err
	if err != nil {
		return fmt.Errorf("发送短信失败: %w", err)
	}
	return nil
}
