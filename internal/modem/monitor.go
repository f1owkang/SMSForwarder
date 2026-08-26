package modem

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"smsforwarder/internal/channel"
	"smsforwarder/internal/logging"
)

const (
	dest           = "org.freedesktop.ModemManager1"
	rootPath       = "/org/freedesktop/ModemManager1"
	ifaceMessaging = "org.freedesktop.ModemManager1.Modem.Messaging"
	ifaceSmsProps  = "org.freedesktop.ModemManager1.Sms"
	stateReceived  = 3
	pollBudget     = 5 * time.Second
	pollTick       = 1 * time.Second
	workerCount    = 4
	jobQueue       = 64
)

type smsJob struct {
	modemPath dbus.ObjectPath
	smsPath   dbus.ObjectPath
}

type Monitor struct {
	conn       *dbus.Conn
	onSMS      func(channel.Message) bool
	log        *logging.Logger
	autoDelete bool
	jobs       chan smsJob
}

func NewMonitor(conn *dbus.Conn, onSMS func(channel.Message) bool, lg *logging.Logger, autoDelete bool) *Monitor {
	return &Monitor{conn: conn, onSMS: onSMS, log: lg, autoDelete: autoDelete}
}

func modemPathsFromObjects(objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant) []dbus.ObjectPath {
	var out []dbus.ObjectPath
	for p, ifaces := range objs {
		if _, ok := ifaces["org.freedesktop.ModemManager1.Modem"]; ok {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func enumerateModems(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := conn.Object(dest, rootPath).Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs)
	if err != nil {
		return nil, fmt.Errorf("枚举调制解调器失败: %w", err)
	}
	return modemPathsFromObjects(objs), nil
}

func FirstModemPath(conn *dbus.Conn) (dbus.ObjectPath, error) {
	paths, err := enumerateModems(conn)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("未发现任何调制解调器")
	}
	return paths[0], nil
}

func parseMMTime(now time.Time, s string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.Local().Format("2006-01-02 15:04:05")
		}
	}
	return now.Format("2006-01-02 15:04:05")
}

func awaitReceived(get func() (uint32, error), budget, tick time.Duration, sleep func(time.Duration)) (bool, error) {
	for i := 0; ; i++ {
		st, err := get()
		if err != nil {
			return false, err
		}
		if st == stateReceived {
			return true, nil
		}
		if time.Duration(i+1)*tick >= budget {
			return false, nil
		}
		sleep(tick)
	}
}

// subscribe 只在启动时调用一次。D-Bus 匹配规则属于本连接，ModemManager
// 重启后仍然有效，因此重启时无需也不应重复 AddMatchSignal（重复订阅会导致
// 每条信号被投递 N 次）。
func (mo *Monitor) subscribe() error {
	if err := mo.conn.AddMatchSignal(
		dbus.WithMatchInterface(ifaceMessaging),
		dbus.WithMatchMember("Added"),
		dbus.WithMatchSender(dest),
	); err != nil {
		return fmt.Errorf("订阅短信信号失败: %w", err)
	}
	if err := mo.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, dest),
	); err != nil {
		return fmt.Errorf("订阅名称变更信号失败: %w", err)
	}
	_, err := FirstModemPath(mo.conn)
	return err
}

func (mo *Monitor) Run(ctx context.Context) error {
	sigCh := make(chan *dbus.Signal, 16)
	mo.conn.Signal(sigCh)
	defer mo.conn.RemoveSignal(sigCh)
	if err := mo.subscribe(); err != nil {
		return err
	}
	jobs := make(chan smsJob, jobQueue)
	mo.jobs = jobs
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				mo.handleIncoming(j.modemPath, j.smsPath)
			}
		}()
	}
	defer func() {
		close(jobs)
		wg.Wait()
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-sigCh:
			mo.dispatch(sig)
		}
	}
}

func (mo *Monitor) dispatch(sig *dbus.Signal) {
	switch {
	case sig.Name == ifaceMessaging+".Added":
		path, ok := sig.Body[0].(dbus.ObjectPath)
		received, ok2 := sig.Body[1].(bool)
		if !ok || !ok2 || !received {
			return
		}
		mo.jobs <- smsJob{modemPath: sig.Path, smsPath: path}
	case sig.Name == "org.freedesktop.DBus.NameOwnerChanged":
		mo.log.Warn("ModemManager 发生变更，重新枚举调制解调器")
		if _, err := FirstModemPath(mo.conn); err != nil {
			mo.log.Error(fmt.Sprintf("重新枚举调制解调器失败: %v", err))
		}
	}
}

func (mo *Monitor) handleIncoming(modemPath, path dbus.ObjectPath) {
	defer func() {
		if r := recover(); r != nil {
			mo.log.Error(fmt.Sprintf("处理短信异常（已恢复）: %v", r))
		}
	}()
	obj := mo.conn.Object(dest, path)
	get := func() (uint32, error) {
		var props map[string]dbus.Variant
		if err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, ifaceSmsProps).Store(&props); err != nil {
			return 0, err
		}
		v, ok := props["State"].Value().(uint32)
		if !ok {
			return 0, fmt.Errorf("State 属性缺失")
		}
		return v, nil
	}
	received, err := awaitReceived(get, pollBudget, pollTick, time.Sleep)
	if err != nil {
		mo.log.Error(fmt.Sprintf("读取短信状态失败: %v", err))
		return
	}
	if !received {
		mo.log.Warn("等待短信就绪超时，放弃本次处理")
		return
	}
	var props map[string]dbus.Variant
	if err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, ifaceSmsProps).Store(&props); err != nil {
		mo.log.Error(fmt.Sprintf("读取短信属性失败: %v", err))
		return
	}
	number := variantString(props, "Number", "未知号码")
	text := strings.TrimSpace(variantString(props, "Text", ""))
	if text == "" {
		mo.log.Warn(fmt.Sprintf("空短信内容，跳过 path: %s", path))
		return
	}
	msg := channel.Message{
		Number:    number,
		Text:      text,
		Timestamp: parseMMTime(time.Now(), variantString(props, "Timestamp", "")),
		ModemPath: string(modemPath),
	}
	if mo.autoDelete && mo.onSMS(msg) {
		mo.delete(modemPath, path)
	}
}

func variantString(props map[string]dbus.Variant, key, def string) string {
	if v, ok := props[key].Value().(string); ok {
		return v
	}
	return def
}

func (mo *Monitor) delete(modemPath, smsPath dbus.ObjectPath) {
	err := mo.conn.Object(dest, modemPath).CallWithContext(context.Background(),
		ifaceMessaging+".Delete", 0, smsPath).Err
	if err != nil {
		mo.log.Error(fmt.Sprintf("删除短信失败: %s - %v", smsPath, err))
		return
	}
	mo.log.Info(fmt.Sprintf("已删除短信: %s", smsPath))
}
