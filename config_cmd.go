package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// editorCommands returns the commands to try, in order, to open path for editing.
func editorCommands(path string) []*exec.Cmd {
	var cmds []*exec.Cmd
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if fields := strings.Fields(os.Getenv(env)); len(fields) > 0 {
			cmds = append(cmds, exec.Command(fields[0], append(fields[1:], path)...))
			break
		}
	}
	switch runtime.GOOS {
	case "windows":
		cmds = append(cmds, exec.Command("cmd", "/c", "start", "", path), exec.Command("notepad.exe", path))
	case "darwin":
		cmds = append(cmds, exec.Command("open", path))
	default:
		cmds = append(cmds, exec.Command("xdg-open", path))
	}
	return cmds
}

// configCommand prints the path of config.json (creating it if missing) and
// opens it in an editor unless --path is given.
func configCommand(root string, args []string) error {
	flags := flag.NewFlagSet("config", flag.ExitOnError)
	pathOnly := flags.Bool("path", false, "only print the path of config.json")
	_ = flags.Parse(args)
	path := filepath.Join(root, "config.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := initConfig(root); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "config created from example")
	}
	fmt.Println(path)
	if *pathOnly {
		return nil
	}
	var last error
	for _, cmd := range editorCommands(path) {
		// Attach the terminal so console editors (EDITOR=vim) work; GUI
		// launchers return immediately.
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if last = cmd.Run(); last == nil {
			return nil
		}
	}
	fmt.Fprintln(os.Stderr, "warning: could not open an editor:", last)
	return nil
}
