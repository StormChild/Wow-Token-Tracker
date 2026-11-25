package autostart

import (
	"encoding/binary"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

type call struct {
	name string
	args []string
}

func recorder(out []byte, err error) (*[]call, Runner) {
	var calls []call
	return &calls, func(name string, args ...string) ([]byte, error) {
		calls = append(calls, call{name, args})
		return out, err
	}
}

const spaced = `C:\Users\Jo Smith\AppData\Local\Programs\WowTokenTracker\WowTokenTracker.exe`

func fixedUser() (string, error) { return `PC\Jo Smith`, nil }

func TestTaskXML(t *testing.T) {
	x := TaskXML(`C:\Tom & Jerry\WowTokenTracker.exe`, `PC\Jo Smith`)
	for _, want := range []string{
		`<Command>C:\Tom &amp; Jerry\WowTokenTracker.exe</Command>`,
		`<LogonTrigger>`,
		`<UserId>PC\Jo Smith</UserId>`,
		`<LogonType>InteractiveToken</LogonType>`, // only when user is logged on
		`<RunLevel>LeastPrivilege</RunLevel>`,
		`<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>`,
		`<StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>`,
		`<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>`, // never stop after 3 days
	} {
		if !strings.Contains(x, want) {
			t.Errorf("task XML missing %s", want)
		}
	}
}

func TestEnableCallsSchtasksWithUTF16XML(t *testing.T) {
	var args []string
	var file []byte
	run := func(name string, a ...string) ([]byte, error) {
		if name != "schtasks" {
			t.Errorf("ran %s", name)
		}
		args = a
		file, _ = os.ReadFile(a[4])
		return nil, nil
	}
	if err := (Autostart{Run: run, User: fixedUser}).Enable(spaced); err != nil {
		t.Fatal(err)
	}
	if len(args) != 6 || !reflect.DeepEqual([]string{args[0], args[1], args[2], args[3], args[5]},
		[]string{"/Create", "/TN", "WowTokenTracker", "/XML", "/F"}) {
		t.Fatalf("args = %q", args)
	}
	if len(file) < 2 || file[0] != 0xFF || file[1] != 0xFE {
		t.Fatal("task file is not UTF-16LE with BOM")
	}
	if !strings.Contains(string(decode(file)), "<Command>"+spaced+"</Command>") {
		t.Fatal("task file missing exe path")
	}
	if _, err := os.Stat(args[4]); !os.IsNotExist(err) {
		t.Fatal("temp task file not removed")
	}
}

func TestEnableReportsOutputOnError(t *testing.T) {
	_, run := recorder([]byte("ERROR: Access is denied."), errors.New("exit status 1"))
	err := (Autostart{Run: run, User: fixedUser}).Enable(spaced)
	if err == nil || !strings.Contains(err.Error(), "Access is denied") {
		t.Fatalf("err = %v", err)
	}
}

func TestDisable(t *testing.T) {
	calls, run := recorder(nil, nil)
	if err := (Autostart{Run: run}).Disable(); err != nil {
		t.Fatal(err)
	}
	want := []string{"/Delete", "/TN", "WowTokenTracker", "/F"}
	if !reflect.DeepEqual((*calls)[0].args, want) {
		t.Fatalf("args = %q", (*calls)[0].args)
	}
}

func TestEnabledFalseOnError(t *testing.T) {
	_, run := recorder([]byte("ERROR: The system cannot find the file specified."), errors.New("exit status 1"))
	ok, err := (Autostart{Run: run}).Enabled()
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestEnabledTrue(t *testing.T) {
	_, run := recorder([]byte("TaskName  Next Run Time  Status\nWowTokenTracker  N/A  Ready"), nil)
	if ok, _ := (Autostart{Run: run}).Enabled(); !ok {
		t.Fatal("want enabled")
	}
}

const taskXML = `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Actions Context="Author">
    <Exec>
      <Command>"C:\a b\x.exe"</Command>
    </Exec>
  </Actions>
</Task>`

func utf16le(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := []byte{0xFF, 0xFE}
	for _, r := range u {
		b = binary.LittleEndian.AppendUint16(b, r)
	}
	return b
}

func TestTaskPathParsesXML(t *testing.T) {
	for name, out := range map[string][]byte{"utf16": utf16le(taskXML), "utf8": []byte(taskXML)} {
		_, run := recorder(out, nil)
		got, err := (Autostart{Run: run}).TaskPath()
		if err != nil || got != `C:\a b\x.exe` {
			t.Errorf("%s: got %q err %v", name, got, err)
		}
	}
}

func TestTaskPathEscapedQuotes(t *testing.T) {
	x := `<Task><Actions><Exec><Command>&quot;C:\a b\x.exe&quot;</Command></Exec></Actions></Task>`
	_, run := recorder([]byte(x), nil)
	if got, _ := (Autostart{Run: run}).TaskPath(); got != `C:\a b\x.exe` {
		t.Fatalf("got %q", got)
	}
}
