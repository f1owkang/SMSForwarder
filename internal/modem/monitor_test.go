package modem

import (
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestModemPathsFromObjects(t *testing.T) {
	objs := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{
		"/org/freedesktop/ModemManager1/Modem/0": {"org.freedesktop.ModemManager1.Modem": {}},
		"/org/freedesktop/ModemManager1/Modem/2": {"org.freedesktop.ModemManager1.Modem": {}},
		"/org/freedesktop/ModemManager1/SMS/7":   {"org.freedesktop.ModemManager1.Sms": {}},
		"/org/freedesktop/DBus":                  {"org.freedesktop.DBus": {}},
	}
	got := modemPathsFromObjects(objs)
	if len(got) != 2 || got[0] != "/org/freedesktop/ModemManager1/Modem/0" || got[1] != "/org/freedesktop/ModemManager1/Modem/2" {
		t.Fatalf("应只含排序后的 Modem 对象: %v", got)
	}
	if got := modemPathsFromObjects(nil); len(got) != 0 {
		t.Fatal("空对象应得空列表")
	}
}

func TestAwaitReceivedImmediate(t *testing.T) {
	calls := 0
	ok, err := awaitReceived(func() (uint32, error) { calls++; return 3, nil },
		5*time.Second, time.Second, func(time.Duration) {})
	if err != nil || !ok || calls != 1 {
		t.Fatalf("立即收到应 ok: ok=%v err=%v calls=%d", ok, err, calls)
	}
}

func TestAwaitReceivedTimeout(t *testing.T) {
	slept := 0
	ok, err := awaitReceived(func() (uint32, error) { return 1, nil }, 5*time.Second, time.Second,
		func(time.Duration) { slept++ })
	if err != nil || ok {
		t.Fatalf("超时应返回 false: ok=%v err=%v", ok, err)
	}
	if slept != 4 {
		t.Fatalf("budget=5s tick=1s 应回退等待 4 次: %d", slept)
	}
}

func TestAwaitReceivedGetterError(t *testing.T) {
	want := errors.New("dbus down")
	_, err := awaitReceived(func() (uint32, error) { return 0, want }, 5*time.Second, time.Second, func(time.Duration) {})
	if !errors.Is(err, want) {
		t.Fatalf("getter 错误应透传: %v", err)
	}
}

func TestParseMMTimeVariants(t *testing.T) {
	orig := time.Local
	time.Local = time.FixedZone("CST", 8*3600)
	defer func() { time.Local = orig }()
	fake := time.Date(2026, 8, 26, 0, 0, 0, 0, time.Local)
	cases := []struct{ in, want string }{
		{"2026-01-02T03:04:05+08:00", "2026-01-02 03:04:05"},
		{"2026-01-02T03:04:05", "2026-01-02 03:04:05"},
		{"garbage", "2026-08-26 00:00:00"},
	}
	for _, c := range cases {
		if got := parseMMTime(fake, c.in); got != c.want {
			t.Fatalf("parseMMTime(%q)=%q 期望 %q", c.in, got, c.want)
		}
	}
}
