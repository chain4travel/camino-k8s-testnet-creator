package pkg

import (
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"
)

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	stdout, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		return "unknown"
	}

	return strings.TrimSpace(string(stdout))
}()
