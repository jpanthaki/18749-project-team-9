package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Relaunch(method string, id int, serverAddrs map[string]string) {
	cwd, _ := os.Getwd()
	parent := filepath.Dir(cwd)

	command := fmt.Sprintf(`./run.sh server %s %d %s %s %s`, method, id, serverAddrs["S1"], serverAddrs["S2"], serverAddrs["S3"])

	escaped := strings.ReplaceAll(command, `"`, `/"`)

	appleScript := fmt.Sprintf(`
		tell application "Terminal"
			do script "cd '%s'; %s"
			activate
		end tell
		`, parent, escaped)

	cmd := exec.Command("osascript", "-e", appleScript)

	cmd.Output()
}
