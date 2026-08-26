package modem

import (
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"smsforwarder/internal/channel"
)

func TestDispatchAddedSubmitsJob(t *testing.T) {
	mo := &Monitor{jobs: make(chan smsJob, 1)}
	sig := &dbus.Signal{
		Name: ifaceMessaging + ".Added",
		Path: "/org/freedesktop/ModemManager1/Modem/0",
		Body: []any{dbus.ObjectPath("/org/freedesktop/ModemManager1/SMS/3"), true},
	}
	done := make(chan struct{})
	go func() {
		mo.dispatch(sig)
		close(done)
	}()
	select {
	case j := <-mo.jobs:
		if j.modemPath != "/org/freedesktop/ModemManager1/Modem/0" || j.smsPath != "/org/freedesktop/ModemManager1/SMS/3" {
			t.Fatalf("job 应携带收信 modem 与短信 path: %+v", j)
		}
	case <-time.After(time.Second):
		t.Fatal("Added 短信未投递到队列")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatch 未返回")
	}
}

func TestDispatchSkipsNotReceived(t *testing.T) {
	mo := &Monitor{jobs: make(chan smsJob, 1)}
	sig := &dbus.Signal{
		Name: ifaceMessaging + ".Added",
		Path: "/org/freedesktop/ModemManager1/Modem/0",
		Body: []any{dbus.ObjectPath("/org/freedesktop/ModemManager1/SMS/3"), false},
	}
	mo.dispatch(sig)
	select {
	case j := <-mo.jobs:
		t.Fatalf("本地新增/未接收的短信不应投递: %+v", j)
	default:
	}
}

func TestDispatchIgnoresOtherSignals(t *testing.T) {
	mo := &Monitor{jobs: make(chan smsJob, 1)}
	sig := &dbus.Signal{Name: "org.freedesktop.DBus.NameAcquired", Path: "/org/freedesktop/DBus"}
	mo.dispatch(sig)
	select {
	case j := <-mo.jobs:
		t.Fatalf("无关信号不应投递: %+v", j)
	default:
	}
}

func TestNewMonitorPollBudgetDefault(t *testing.T) {
	if mo := NewMonitor(nil, nil, nil, false, 0); mo.pollBudget != 5*time.Second {
		t.Fatalf("pollBudget 缺省应为 5s: %v", mo.pollBudget)
	}
	if mo := NewMonitor(nil, nil, nil, false, 10*time.Second); mo.pollBudget != 10*time.Second {
		t.Fatalf("显式 pollBudget 应采用: %v", mo.pollBudget)
	}
}

const (
	testModemPath = "/org/freedesktop/ModemManager1/Modem/0"
	testSMSPath   = "/org/freedesktop/ModemManager1/SMS/1"
)

func TestDeliverForwardsEvenWhenAutoDeleteDisabled(t *testing.T) {
	calls := 0
	mo := &Monitor{
		onSMS: func(channel.Message) bool { calls++; return true },
		deleteSMS: func(dbus.ObjectPath, dbus.ObjectPath) {
			t.Fatal("auto_delete=false 时不应删除")
		},
	}
	mo.deliver(channel.Message{}, testModemPath, testSMSPath)
	if calls != 1 {
		t.Fatalf("转发回调应恰好调用一次（auto_delete=false 不得短路转发）: %d", calls)
	}
}

func TestDeliverDeletesWhenAutoDeleteAndDelivered(t *testing.T) {
	var deleted []dbus.ObjectPath
	forwarded := false
	mo := &Monitor{
		autoDelete: true,
		onSMS: func(channel.Message) bool {
			forwarded = true
			return true
		},
		deleteSMS: func(modemPath, smsPath dbus.ObjectPath) {
			deleted = append(deleted, modemPath, smsPath)
		},
	}
	mo.deliver(channel.Message{}, testModemPath, testSMSPath)
	if !forwarded || len(deleted) != 2 ||
		deleted[0] != testModemPath || deleted[1] != testSMSPath {
		t.Fatalf("转发成功后应按收信 modem 删除: forwarded=%v deleted=%v", forwarded, deleted)
	}
}

func TestDeliverSkipsDeleteWhenNotDelivered(t *testing.T) {
	mo := &Monitor{
		autoDelete: true,
		onSMS:      func(channel.Message) bool { return false },
		deleteSMS: func(dbus.ObjectPath, dbus.ObjectPath) {
			t.Fatal("转发未成功不应删除")
		},
	}
	mo.deliver(channel.Message{}, testModemPath, testSMSPath)
}

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
