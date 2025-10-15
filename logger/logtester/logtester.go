package main

import (
	"18749-team9/logger" // <-- update this import path to match your module name
	"fmt"
)

func main() {
	fmt.Println()
	fmt.Println("🧪 Logger Color Test — showing all defined colors")
	fmt.Println("───────────────────────────────────────────────")

	// Iterate through each component and its defined colors
	for component, colorMap := range logger.ComponentColors() {
		fmt.Printf("\n🔹 Component: %s\n", component)
		fmt.Println("──────────────────────────────")

		l := logger.New(component)

		for msgType, _ := range colorMap {
			// format a descriptive message
			msg := fmt.Sprintf("%s event (%s)", msgType, component)
			// manually print the color being used
			l.Log(msg, msgType)
		}
	}

	fmt.Println("\n✅ Done — verify all colors look good in your terminal.\n")
}
