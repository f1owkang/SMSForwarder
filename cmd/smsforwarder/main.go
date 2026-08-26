package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"smsforwarder/internal/channel"
	"smsforwarder/internal/config"
	"smsforwarder/internal/extract"
	"smsforwarder/internal/logging"
	"smsforwarder/internal/modem"
)

var version = "dev"

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[FATAL] "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	cfgPath := flag.String("c", "", "配置文件路径")
	doCheck := flag.Bool("check", false, "校验配置后退出")
	doTest := flag.Bool("test", false, "向所有接收者发送测试消息")
	showVer := flag.Bool("v", false, "打印版本号")
	flag.Parse()

	if *showVer {
		fmt.Println(version)
		return
	}

	path, err := config.ResolvePath(*cfgPath)
	if err != nil {
		fatalf("%v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		fatalf("%v", err)
	}
	if *doCheck {
		for _, name := range []string{"stopwords.txt", "userwords.txt"} {
			if config.FindWordlist(path, name) == "" {
				fmt.Printf("警告: 未找到词库文件 %s（运行时将降级使用内置词典）\n", name)
			}
		}
		fmt.Printf("✓ 配置校验通过: %s（接收者 %d 个）\n", path, len(cfg.Recipients))
		return
	}

	lg := setupLogging(cfg.LogFile)

	ext, err := extract.New(config.FindWordlist(path, "stopwords.txt"), config.FindWordlist(path, "userwords.txt"))
	if err != nil {
		fatalf("%v", err)
	}
	for _, w := range ext.Warnings() {
		lg.Warn(w)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		fatalf("连接系统总线失败: %v", err)
	}

	hc := channel.NewHTTPClient()
	app, err := NewApp(cfg, lg, ext, hc, conn)
	if err != nil {
		fatalf("%v", err)
	}

	if *doTest {
		fmt.Fprintln(os.Stderr, "[警告] -test 会对每个接收者的每个渠道真实推送测试消息，sms 渠道将产生真实短信资费。")
		os.Exit(app.RunSelfTest())
	}

	startHeartbeat(cfg.Heartbeat.Duration, lg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mon := modem.NewMonitor(conn, app.HandleSMS, lg, cfg.AutoDelete, cfg.PollBudget.Duration)
	lg.Info("短信监听服务已启动")
	if err := mon.Run(ctx); err != nil {
		fatalf("%v", err)
	}
	lg.Info("服务已退出")
}

func setupLogging(logFile string) *logging.Logger {
	var f *os.File
	if logFile != "" {
		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
			fatalf("创建日志目录失败: %v", err)
		}
		out, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fatalf("打开日志文件失败: %v", err)
		}
		f = out
	}
	return logging.New(os.Stdout, f)
}

func startHeartbeat(d time.Duration, lg *logging.Logger) {
	if d <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for range ticker.C {
			lg.Info("心跳正常，服务运行中")
		}
	}()
}
