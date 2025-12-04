package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Relaunch(method string, id string) {
	cwd, _ := os.Getwd()

	data, _ := os.ReadFile(fmt.Sprintf("%s/serverAddrs.json", cwd))

	serverAddrs := make(map[string]string)

	json.Unmarshal(data, &serverAddrs)

	fmt.Println(serverAddrs)

	command := fmt.Sprintf(`./run.sh server %s %s %s %s %s`, method, id[1:], serverAddrs["S1"], serverAddrs["S2"], serverAddrs["S3"])

	escaped := strings.ReplaceAll(command, `"`, `/"`)

	appleScript := fmt.Sprintf(`
		tell application "Terminal"
			do script "cd '%s'; %s"
			activate
		end tell
		`, cwd, escaped)

	cmd := exec.Command("osascript", "-e", appleScript)

	cmd.Output()
}
