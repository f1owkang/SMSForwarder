package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "/etc/smsforwarder/config.yml"

var (
	phoneRe = regexp.MustCompile(`^\+?\d{5,20}$`)
	urlRe   = regexp.MustCompile(`^https?://\S+$`)
)

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("时长格式非法 %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

type ChannelConfig struct {
	Type      string `yaml:"type"`
	Token     string `yaml:"token"`
	Phone     string `yaml:"phone"`
	Server    string `yaml:"server"`
	DeviceKey string `yaml:"device_key"`
	SendKey   string `yaml:"send_key"`
	BotToken  string `yaml:"bot_token"`
	ChatID    string `yaml:"chat_id"`
	Proxy     string `yaml:"proxy"`
	URL       string `yaml:"url"`
}

type Recipient struct {
	Name     string          `yaml:"name"`
	Channels []ChannelConfig `yaml:"channels"`
}

type Config struct {
	LogFile    string      `yaml:"log_file"`
	AutoDelete bool        `yaml:"auto_delete"`
	Heartbeat  *Duration   `yaml:"heartbeat"`
	PollBudget *Duration   `yaml:"poll_timeout"`
	Recipients []Recipient `yaml:"recipients"`

	path string
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析配置失败 %s: %w", path, err)
	}
	c.path = path
	if c.Heartbeat == nil {
		c.Heartbeat = &Duration{Duration: 24 * time.Hour}
	}
	if c.PollBudget == nil {
		c.PollBudget = &Duration{Duration: 5 * time.Second}
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Dir() string { return filepath.Dir(c.path) }

func (c *Config) Validate() error {
	if len(c.Recipients) == 0 {
		return errors.New("recipients 不能为空，至少配置一个接收者")
	}
	if c.PollBudget == nil || c.PollBudget.Duration <= 0 {
		return errors.New("poll_timeout 必须大于 0")
	}
	seen := map[string]bool{}
	for i, r := range c.Recipients {
		where := fmt.Sprintf("接收者 #%d", i+1)
		if r.Name != "" {
			where = fmt.Sprintf("接收者 %q", r.Name)
			if seen[r.Name] {
				return fmt.Errorf("%s: 名称重复", where)
			}
			seen[r.Name] = true
		}
		if len(r.Channels) == 0 {
			return fmt.Errorf("%s: channels 不能为空", where)
		}
		for j, ch := range r.Channels {
			if err := validateChannel(ch); err != nil {
				return fmt.Errorf("%s 渠道 #%d (%s): %w", where, j+1, ch.Type, err)
			}
		}
	}
	return nil
}

func validateChannel(ch ChannelConfig) error {
	switch ch.Type {
	case "pushplus":
		if ch.Token == "" {
			return errors.New("token 不能为空")
		}
	case "serverchan":
		if ch.SendKey == "" {
			return errors.New("send_key 不能为空")
		}
	case "bark":
		if ch.DeviceKey == "" {
			return errors.New("device_key 不能为空")
		}
		if ch.Server != "" && !urlRe.MatchString(ch.Server) {
			return fmt.Errorf("server 非法: %q", ch.Server)
		}
	case "telegram":
		if ch.BotToken == "" || ch.ChatID == "" {
			return errors.New("bot_token 与 chat_id 不能为空")
		}
	case "webhook":
		if !urlRe.MatchString(ch.URL) {
			return fmt.Errorf("url 非法: %q", ch.URL)
		}
	case "sms":
		if !phoneRe.MatchString(ch.Phone) {
			return fmt.Errorf("phone 非法: %q", ch.Phone)
		}
	default:
		return fmt.Errorf("未知渠道类型 %q", ch.Type)
	}
	return nil
}

func ResolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		if _, err := os.Stat(flagPath); err != nil {
			return "", fmt.Errorf("配置文件不存在: %s", flagPath)
		}
		return flagPath, nil
	}
	for _, p := range []string{DefaultPath, "config.yml"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到配置文件（尝试过 %s 与 ./config.yml），可用 -c 指定路径", DefaultPath)
}

func FindWordlist(configPath, name string) string {
	dirs := []string{filepath.Dir(configPath), "/etc/smsforwarder"}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
