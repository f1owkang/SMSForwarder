package channel

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestSMSCreateProps(t *testing.T) {
	props := smsCreateProps("18100000000", "转发内容")
	num, ok := props["number"].Value().(string)
	if !ok || num != "18100000000" {
		t.Fatalf("number 属性错误: %v", props["number"])
	}
	txt, ok := props["text"].Value().(string)
	if !ok || txt != "转发内容" {
		t.Fatalf("text 属性错误: %v", props["text"])
	}
}

func TestSMSChannelMeta(t *testing.T) {
	s := &SMS{}
	if s.Name() != "sms" {
		t.Fatalf("渠道名错误: %q", s.Name())
	}
	_ = dbus.SystemBus
}
