package extract

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-ego/gse"
	"github.com/go-ego/gse/hmm/extracker"
)

var codeRe = regexp.MustCompile(`(?i)(验证码|校验码|动态码|动态密码|\b(?:verification code|code|otp)\b)[^\d]{0,10}?(\d{4,6})`)

type Extractor struct {
	seg       gse.Segmenter
	stopwords map[string]bool
	extractFn func(text string, n int) []string
	warnings  []string
}

func New(stopwordsPath, userdictPath string) (*Extractor, error) {
	seg, err := gse.NewEmbed("zh_s", "alpha")
	if err != nil {
		return nil, fmt.Errorf("加载内置词典失败: %w", err)
	}
	e := &Extractor{seg: seg, stopwords: map[string]bool{}}
	if userdictPath != "" {
		if loadErr := e.seg.LoadDict(userdictPath); loadErr != nil {
			e.warnf("用户词典加载失败 %s: %v", userdictPath, loadErr)
		}
	}
	if stopwordsPath != "" {
		if _, statErr := os.Stat(stopwordsPath); os.IsNotExist(statErr) {
			e.warnf("停用词文件缺失 %s，关键词质量可能下降", stopwordsPath)
		} else {
			data, readErr := os.ReadFile(stopwordsPath)
			if readErr != nil {
				return nil, fmt.Errorf("读取停用词失败: %w", readErr)
			}
			for _, line := range strings.Split(string(data), "\n") {
				w := strings.TrimSpace(line)
				if w != "" {
					e.stopwords[w] = true
				}
			}
		}
	}
	te := &extracker.TagExtracter{}
	te.WithGse(e.seg)
	_ = te.LoadIdf()
	e.extractFn = func(text string, n int) []string {
		tags := te.ExtractTags(text, n)
		words := make([]string, len(tags))
		for i, tag := range tags {
			words[i] = tag.Text
		}
		return words
	}
	return e, nil
}

func (e *Extractor) warnf(format string, args ...any) {
	e.warnings = append(e.warnings, fmt.Sprintf(format, args...))
}

func (e *Extractor) Warnings() []string { return e.warnings }

func (e *Extractor) Extract(text string) string {
	if code := matchCode(text); code != "" {
		return code
	}
	if e.extractFn == nil {
		return ""
	}
	var out []string
	for _, w := range e.extractFn(text, 10) {
		if utf8.RuneCountInString(w) < 2 || isNumber(w) || e.stopwords[w] {
			continue
		}
		out = append(out, w)
		if len(out) == 4 {
			break
		}
	}
	return strings.Join(out, "、")
}

func matchCode(text string) string {
	for _, loc := range codeRe.FindAllStringSubmatchIndex(text, -1) {
		end := loc[1]
		if end == len(text) || text[end] < '0' || text[end] > '9' {
			return fmt.Sprintf("验证码【%s】", text[loc[4]:loc[5]])
		}
	}
	return ""
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
