// Package opener opens a URL/path using the OS's default handler, so the
// `o` keybinding works without depending on a browser library.
package opener

import (
	"os/exec"
	"runtime"
)

// Open launches target (typically a URL, but works for local paths too)
// with the platform's default opener.
func Open(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		// "cmd /c start" needs an empty title arg before the URL, otherwise
		// a URL containing certain characters is misparsed as the title.
		cmd = exec.Command("cmd", "/c", "start", "", target)
	default: // linux and other unix-likes
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
