package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestClockZoneSwitches checks the timestamp page through the shell: it
// starts in the system's own zone, and another one can be chosen with how
// far it is from UTC shown beside it.
func TestClockZoneSwitches(t *testing.T) {
	a := New()
	a.selected = "clock"
	tr := ui.NewTester(a.View, 980, 700)

	local := ""
	for _, s := range tr.Texts() {
		if strings.HasPrefix(s, "本地 ") {
			local = s
		}
	}
	if local == "" {
		t.Fatalf("时区选择框没有显示本地时区，得到 %q", tr.Texts())
	}
	if err := tr.Click(local); err != nil {
		t.Fatal(err)
	}
	// Japan keeps the same offset all year, so this reads the same whenever
	// the test runs.
	if err := tr.Click("Asia/Tokyo UTC+09:00"); err != nil {
		t.Fatal(err)
	}
	if !tr.HasText("Asia/Tokyo UTC+09:00") {
		t.Error("选择框应当显示所选的时区与它的偏移")
	}
	if !tr.HasText("JST") {
		t.Error("切换到东京后应当按 JST 显示时间")
	}
}
