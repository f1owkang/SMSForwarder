package channel

import "context"

type Message struct {
	Number    string
	Text      string
	Keyword   string
	Timestamp string
}

type Channel interface {
	Name() string
	Send(ctx context.Context, m Message) error
}

func Title(m Message) string {
	if m.Keyword != "" {
		return m.Keyword
	}
	return m.Number
}

func Body(m Message) string {
	return m.Number + "\n" + m.Text + "\n" + m.Timestamp
}
