//go:build ignore

package main

import (
	"fmt"
	"os"
	"github.com/forge/sword/internal/org"
)

func main() {
	os.Chdir("/tmp/forge-test3")
	o := org.New("TestOrg", "human", ".forge/org.json")
	result, err := o.Bootstrap()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Printf("divisions: %d, agents: %d\n", len(result.Divisions), len(result.Agents))
	status := o.GetStatus()
	fmt.Printf("version: %d, active_divs: %d\n", status.Version, status.ActiveDivisions)
	_, ferr := os.Stat("/tmp/forge-test3/.forge/org.json")
	fmt.Printf("file exists: %v\n", ferr == nil)
}
