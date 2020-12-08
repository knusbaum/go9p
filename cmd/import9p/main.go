package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	verbose := flag.Bool("v", false, "Makes the 9p protocol verbose, printing all incoming and outgoing messages.")
	sshPort := flag.Int("p", 22, "The SSH Port to connect to")
	exportPath := flag.String("export-path", "", "The path to the export9p binary on the remote system.")
	loginShell := flag.Bool("login", false, "Causes ssh to try to execute a login shell. This is useful for loading the user's profile and environment (namely the PATH variable). This works when the user's shell is bash, or zsh.")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] user@address:path localmountpoint\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}
	remoteParts := strings.Split(flag.Arg(0), ":")
	localPath := flag.Arg(1)
	if len(remoteParts) != 2 {
		fmt.Fprintf(flag.CommandLine.Output(), "Bad remote address: %s", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
	exportCommand := "export9p"
	if *exportPath != "" {
		exportCommand = *exportPath
	}

	// This shell dance is necessary to pick up the user's profile for PATH and other environment variables.
	args := []string{remoteParts[0], "-p", strconv.Itoa(*sshPort), "exec", exportCommand, "-s", "-noperm", "-dir", remoteParts[1]}
	if *loginShell {
		args = []string{remoteParts[0], "-p", strconv.Itoa(*sshPort), "exec", "$SHELL", "--login", "-c"}
		if *verbose {
			args = append(args, fmt.Sprintf("'export9p -v -s -noperm -dir %s'", remoteParts[1]))
		} else {
			args = append(args, fmt.Sprintf("'export9p -s -noperm -dir %s'", remoteParts[1]))
		}
	}
	sshProc := exec.Command("ssh", args...)
	exportIn, err := sshProc.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get STDIN: %s", err)
	}
	exportOut, err := sshProc.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get STDOUT: %s", err)
	}
	sshProc.Stderr = os.Stderr

	args = []string{"-s", localPath}
	if *verbose {
		args = []string{"-s", "-v", localPath}
	}
	mountProc := exec.Command("mount9p", args...)
	mountProc.Stdin = exportOut
	mountProc.Stdout = exportIn
	mountProc.Stderr = os.Stderr

	done := make(chan struct{})
	go func() {
		mountProc.Start()
		mountProc.Wait()
		done <- struct{}{}
	}()
	go func() {
		sshProc.Start()
		sshProc.Wait()
		done <- struct{}{}
	}()
	defer func() { mountProc.Process.Kill() }()
	defer func() { sshProc.Process.Kill() }()
	<-done
}
