package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/api/icons"
	commonsText "github.com/flanksource/commons/text"
)

func HumanizeBytes(bytes int64) Text {
	return Text{
		Content: commonsText.HumanizeBytes(bytes),
	}
}

func HumanDate(d any, format string) Text {
	if format == "" {
		format = time.RFC3339
	}
	switch t := d.(type) {
	case time.Time:
		return Text{
			Content: t.Format(format),
			Style:   "date",
		}
	case *time.Time:
		return Text{
			Content: t.Format(format),
			Style:   "date",
		}
	}
	return Text{
		Content: fmt.Sprintf("%v", d),
		Style:   "date",
	}
}

func Human(content any, styles ...string) Text {
	switch t := content.(type) {
	case Text:
		return t
	case Textable:
		return Text{}.Add(t)
	case time.Time:
		return Text{
			Content: t.Format(time.RFC3339),
			Style:   strings.Join(append(styles, "date"), " "),
		}
	case *time.Time:
		return Text{
			Content: t.Format(time.RFC3339),
			Style:   strings.Join(append(styles, "date"), " "),
		}
	case time.Duration:
		var v string
		if t < 5*time.Second {
			v = fmt.Sprintf("%dms", t.Milliseconds())
		} else if t < 1*time.Minute {
			v = fmt.Sprintf("%.2fs", t.Seconds())
		} else if t < 1*time.Hour {
			v = fmt.Sprintf("%.1fm", t.Minutes())
		} else if t < 24*time.Hour {
			v = fmt.Sprintf("%.1fh", t.Hours())
		} else {
			v = commonsText.HumanizeDuration(t)
		}
		return Text{
			Content: v,
			Style:   strings.Join(append(styles, "duration"), " "),
		}
	case *time.Duration:
		return Human(*t, styles...)
	case int64:
		return HumanNumber(t, styles...)
	case int:
		return HumanNumber(int64(t), styles...)
	case int32:
		return HumanNumber(int64(t), styles...)
	case float32, float64:
		return Text{
			Content: fmt.Sprintf("%.2f", t),
			Style:   strings.Join(append(styles, "number"), " "),
		}

	case bool:
		if t {
			return Text{}.Add(icons.Success)
		} else {
			return Text{}.Add(icons.Fail)
		}
	}

	return Text{Content: fmt.Sprintf("%v", content), Style: strings.Join(styles, " ")}
}

var K = int64(1000)
var M = K * K
var B = M * K

func HumanNumber(value int64, styles ...string) Text {
	v := fmt.Sprintf("%d", value)
	if value >= B {
		v = fmt.Sprintf("%dB", value/B)
	} else if value >= M {
		v = fmt.Sprintf("%dM", value/M)
	} else if value >= 50*K {
		v = fmt.Sprintf("%d", value/K)
	} else if value >= K {
		v = fmt.Sprintf("%.1fK", float64(value)/float64(K))
	}
	return Text{
		Content: v,
		Style:   strings.Join(append(styles, "number"), " "),
	}
}
