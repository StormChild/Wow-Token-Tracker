package autostart

import (
	"os"
	"testing"
)

// Run with WTT_REAL_SCHTASKS=1 to exercise the real Task Scheduler. It creates and then
// deletes the WowTokenTracker task, so don't run it while autostart is in use.
func TestRealSchtasks(t *testing.T) {
	if os.Getenv("WTT_REAL_SCHTASKS") != "1" {
		t.Skip("set WTT_REAL_SCHTASKS=1 to run")
	}
	a := Default()
	if err := a.Enable(spaced); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Disable() })
	if ok, _ := a.Enabled(); !ok {
		t.Fatal("not enabled after Enable")
	}
	if p, err := a.TaskPath(); err != nil || p != spaced {
		t.Fatalf("TaskPath = %q, %v", p, err)
	}
	if err := a.Disable(); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.Enabled(); ok {
		t.Fatal("still enabled after Disable")
	}
}
