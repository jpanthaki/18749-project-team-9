package main

import "18749-team9/helpers"

func main() {
	// // Get the current working directory of the Go program
	// cwd, _ := os.Getwd()
	// parent := filepath.Dir(cwd)

	// // You can change this to whatever command you want to run in Terminal
	// command := `./run.sh server active 1 127.0.0.1:8081 127.0.0.1:8082 127.0.0.1:8083`

	// // Escape inner quotes so AppleScript doesn't break
	// escaped := strings.ReplaceAll(command, `"`, `\"`)

	// // AppleScript that tells Terminal to open a new window and cd into cwd
	// appleScript := fmt.Sprintf(`
	// 	tell application "Terminal"
	// 		do script "cd '%s'; %s"
	// 		activate
	// 	end tell
	// 	`, parent, escaped)

	// cmd := exec.Command("osascript", "-e", appleScript)
	// out, err := cmd.CombinedOutput()

	// if err != nil {
	// 	log.Printf("osascript error: %v\n", err)
	// }
	// log.Printf("osascript output: %s\n", out)

	serverAddrs := map[string]string{
		"S1": "127.0.0.1:8081",
		"S2": "172.26.80.200:8082",
		"S3": "127.0.0.1:8083",
	}

	helpers.Relaunch("active", 1, serverAddrs)
}
