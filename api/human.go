package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/api/icons"
	commonsText "github.com/flanksource/commons/text"
	"github.com/google/uuid"
)

var K = int64(1000)
var M = K * K
var B = M * K

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
		if t == nil {
			return Text{Style: "date"}
		}
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
	if content == nil || IsEmpty(content) {
		return Text{}
	}
	switch t := content.(type) {

	case Text:
		return t
	case Textable:
		return Text{}.Add(t)
	case time.Time:
		if t.Truncate(time.Hour * 24).Equal(t) {
			return Text{
				Content: t.Format("2006-01-02"),
				Style:   strings.Join(append(styles, "date"), " "),
			}
		}
		// Only omit timezone if it's UTC
		if t.Location() == time.UTC {
			return Text{
				Content: t.Format("2006-01-02 15:04:05"),
				Style:   strings.Join(append(styles, "date"), " "),
			}
		}
		return Text{
			Content: t.Format(time.RFC3339),
			Style:   strings.Join(append(styles, "date"), " "),
		}
	case *time.Time:
		if t == nil {
			return Text{}
		}
		return Human(*t, styles...)
	case uuid.UUID:
		if t == uuid.Nil {
			return Text{}
		}
		return Text{Content: t.String(), Style: strings.Join(styles, " ")}
	case *uuid.UUID:
		if t == nil || *t == uuid.Nil {
			return Text{}
		}
		return Human(*t, styles...)
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
		if t == nil {
			return Text{}
		}
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

func TimeAgo(t *time.Time) Text {
	if t == nil {
		return Text{}
	}
	d := time.Since(*t)
	var v string
	switch {
	case d < time.Minute:
		v = fmt.Sprintf("%2ds ago", int(d.Seconds()))
	case d < time.Hour:
		v = fmt.Sprintf("%2dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		v = fmt.Sprintf("%2dh ago", int(d.Hours()))
	default:
		v = fmt.Sprintf("%2dd ago", int(d.Hours()/24))
	}
	return Text{Content: v, Style: "date"}
}

func HumanNumber(value int64, styles ...string) Text {
	v := fmt.Sprintf("%d", value)
	if value >= 50*M {
		v = fmt.Sprintf("%dM", value/M)
	} else if value >= M {
		if value%M < M/10 {
			v = fmt.Sprintf("%dM", value/M)
		} else {
			v = fmt.Sprintf("%.1fM", float64(value)/float64(M))
		}
	} else if value >= 50*K {
		v = fmt.Sprintf("%dK", value/K)
	} else if value >= K {
		if value%K < K/10 {
			v = fmt.Sprintf("%dK", value/K)
		} else {
			v = fmt.Sprintf("%.1fK", float64(value)/float64(K))
		}
	}
	return Text{
		Content: v,
		Style:   strings.Join(append(styles, "number"), " "),
	}
}
