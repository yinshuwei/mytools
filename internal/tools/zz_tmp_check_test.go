package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

func TestTmpCheck(t *testing.T) {
	big := strings.Repeat("QUJDREVG", 1<<20) // 8 MB, one line
	im := &Image{}
	im.raw = big
	im.uri = "data:image/png;base64," + big
	im.path, im.size = "/pics/photo.png", 6<<20
	im.short = [2]string{widgets.Elide(big, previewChars), widgets.Elide(im.uri, previewChars)}

	start := time.Now()
	ui.NewTester(func(c *ui.Context) { im.View(c) }, 1000, 800)
	t.Logf("显示截断后的一帧：%v", time.Since(start))

	im.short = [2]string{big, im.uri}
	start = time.Now()
	ui.NewTester(func(c *ui.Context) { im.View(c) }, 1000, 800)
	t.Logf("显示全文的一帧：%v", time.Since(start))
}
