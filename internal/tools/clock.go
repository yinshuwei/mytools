package tools

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// Clock converts between Unix timestamps and date and time.
type Clock struct {
	unit string // 秒 or 毫秒
	ts   string // a timestamp typed by the user
	dt   string // a date and time typed by the user
}

func (*Clock) ID() string   { return "clock" }
func (*Clock) Name() string { return "时间戳" }
func (*Clock) Desc() string { return "Unix 时间戳与日期时间的互转" }

// layouts are the date and time formats parseTime accepts, best first.
var layouts = []string{
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
	time.RFC3339,
	time.RFC1123Z,
	time.ANSIC,
}

// parseTime reads the date and time the user typed, in the local zone.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, l := range layouts {
		if tt, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return tt, nil
		}
	}
	return time.Parse(time.RFC3339, s)
}

// fromTimestamp reads the timestamp typed by the user.
func (cl *Clock) fromTimestamp() (time.Time, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(cl.ts), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	if cl.unit == "毫秒" {
		return time.UnixMilli(n), nil
	}
	return time.Unix(n, 0), nil
}

func (cl *Clock) View(c *ui.Context) {
	t := c.Theme()
	if cl.unit == "" {
		cl.unit = "秒"
	}
	now := c.Now()
	// Tick twice a second so the clock stays current.
	c.After(500 * time.Millisecond)

	widgets.Field(c, "现在", func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, now.Format("2006-01-02 15:04:05")).FontSize(24).Bold()
			widgets.Hint(c, now.Format("2006-01-02 15:04:05.000 MST")+"　"+now.Format("Mon Jan 2"))
		})
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.CopyButton(c.Key("nows"), "复制秒", strconv.FormatInt(now.Unix(), 10))
		widgets.CopyButton(c.Key("nowms"), "复制毫秒", strconv.FormatInt(now.UnixMilli(), 10))
		widgets.CopyButton(c.Key("nowiso"), "复制 ISO 8601", now.Format(time.RFC3339))
	})

	ui.Divider(c)

	widgets.Field(c, "时间戳 → 日期时间", func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("ts"), &cl.ts).Label("时间戳").Placeholder("1730000000").Width(200)
			ui.Select(c.Key("unit"), &cl.unit, []string{"秒", "毫秒"}).Width(100)
			if ui.Button(c.Key("fill"), "填入当前").Clicked() {
				if cl.unit == "毫秒" {
					cl.ts = strconv.FormatInt(now.UnixMilli(), 10)
				} else {
					cl.ts = strconv.FormatInt(now.Unix(), 10)
				}
			}
		})
	})
	if strings.TrimSpace(cl.ts) != "" {
		if tt, err := cl.fromTimestamp(); err == nil {
			l := tt.Local()
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, l.Format("2006-01-02 15:04:05")).Selectable()
				widgets.Hint(c, l.Format("2006-01-02 15:04:05.000 MST"))
			})
		} else {
			ui.Text(c, "错误：需要一个整数时间戳").FontSize(12).TextColor(t.Danger)
		}
	}

	ui.Divider(c)

	widgets.Field(c, "日期时间 → 时间戳", func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("dt"), &cl.dt).Label("日期时间").Placeholder("2006-01-02 15:04:05").Width(260)
			if ui.Button(c.Key("filldt"), "填入当前").Clicked() {
				cl.dt = now.Format("2006-01-02 15:04:05")
			}
		})
	})
	if strings.TrimSpace(cl.dt) != "" {
		if tt, err := parseTime(cl.dt); err == nil {
			ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "秒 %d", tt.Unix()).Selectable()
				widgets.CopyButton(c.Key("cps"), "复制秒", strconv.FormatInt(tt.Unix(), 10))
				ui.Textf(c, "毫秒 %d", tt.UnixMilli()).Selectable()
				widgets.CopyButton(c.Key("cpms"), "复制毫秒", strconv.FormatInt(tt.UnixMilli(), 10))
			})
		} else {
			ui.Text(c, "错误：无法解析，试试 2006-01-02 15:04:05").FontSize(12).TextColor(t.Danger)
		}
	}
}
