package autostart

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"unicode/utf16"
)

// TaskName is the Scheduled Task's name.
const TaskName = "WowTokenTracker"

// Runner runs a command and returns its combined output.
type Runner func(name string, args ...string) ([]byte, error)

// Autostart manages the logon Scheduled Task through schtasks.
type Autostart struct {
	Run  Runner
	User func() (string, error) // DOMAIN\user the task belongs to
}

const createNoWindow = 0x08000000

// Default runs schtasks without flashing a console window, for the current user.
func Default() Autostart {
	return Autostart{
		Run: func(name string, args ...string) ([]byte, error) {
			cmd := exec.Command(name, args...)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
			return cmd.CombinedOutput()
		},
		User: func() (string, error) {
			u, err := user.Current()
			if err != nil {
				return "", err
			}
			return u.Username, nil
		},
	}
}

// TaskXML is a task definition that starts exePath when userID logs on, only while that user
// is logged on (so the tray icon is visible), without admin rights, on battery, with no time limit.
// A plain `schtasks /SC ONLOGON` needs admin rights, which is why the task is defined in XML.
func TaskXML(exePath, userID string) string {
	esc := func(s string) string {
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	u := esc(userID)
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>Starts WoW Token Tracker in the system tray at logon.</Description></RegistrationInfo>
  <Triggers>
    <LogonTrigger><Enabled>true</Enabled><UserId>` + u + `</UserId></LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author"><UserId>` + u + `</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Enabled>true</Enabled>
  </Settings>
  <Actions Context="Author">
    <Exec><Command>` + esc(exePath) + `</Command></Exec>
  </Actions>
</Task>
`
}

func encodeUTF16(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 2, 2+2*len(u))
	b[0], b[1] = 0xFF, 0xFE
	for _, r := range u {
		b = binary.LittleEndian.AppendUint16(b, r)
	}
	return b
}

func (a Autostart) run(args ...string) ([]byte, error) {
	out, err := a.Run("schtasks", args...)
	if err != nil {
		msg := strings.TrimSpace(string(decode(out)))
		if msg == "" {
			return out, err
		}
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}

// Enable creates or replaces the task for exePath.
func (a Autostart) Enable(exePath string) error {
	userID, err := a.User()
	if err != nil {
		return fmt.Errorf("can't determine Windows user: %w", err)
	}
	f, err := os.CreateTemp("", "wowtokentracker-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, werr := f.Write(encodeUTF16(TaskXML(exePath, userID)))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	_, err = a.run("/Create", "/TN", TaskName, "/XML", f.Name(), "/F")
	return err
}

// Disable removes the task.
func (a Autostart) Disable() error {
	_, err := a.run("/Delete", "/TN", TaskName, "/F")
	return err
}

// Enabled reports whether the task exists. A failing query means it does not.
func (a Autostart) Enabled() (bool, error) {
	if _, err := a.Run("schtasks", "/Query", "/TN", TaskName); err != nil {
		return false, nil
	}
	return true, nil
}

// TaskPath returns the executable the task starts, without surrounding quotes.
func (a Autostart) TaskPath() (string, error) {
	out, err := a.run("/Query", "/TN", TaskName, "/XML")
	if err != nil {
		return "", err
	}
	var task struct {
		Command string `xml:"Actions>Exec>Command"`
	}
	dec := xml.NewDecoder(bytes.NewReader(decode(out)))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil } // already UTF-8
	if err := dec.Decode(&task); err != nil {
		return "", fmt.Errorf("can't read task XML: %w", err)
	}
	return strings.Trim(strings.TrimSpace(task.Command), `"`), nil
}

// decode converts UTF-16LE output (with BOM) to UTF-8; other input is returned unchanged.
func decode(b []byte) []byte {
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xFE {
		return b
	}
	b = b[2:]
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return []byte(string(utf16.Decode(u)))
}
