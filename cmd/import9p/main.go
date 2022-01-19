package main

import (
	"flag"
	"fmt"
	"go/build"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var verbose = flag.Bool("v", false, "Makes the 9p protocol verbose, printing all incoming and outgoing messages.")
var sshPort = flag.Int("p", 22, "The SSH Port to connect to")
var exportPath = flag.String("export-path", "", "The path to the export9p binary on the remote system.")
var loginShell = flag.Bool("login", false, "Causes ssh to try to execute a login shell. This is useful for loading the user's profile and environment (namely the PATH variable). This works when the user's shell is bash, or zsh.")
var k8s = flag.String("k8s-pod", "", "Kubernetes pod name. If defined, rather than dialing an address, exec into the pod and export a directory from it.")

func sshExport() *exec.Cmd {
	remoteParts := strings.Split(flag.Arg(0), ":")

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
			args = append(args, fmt.Sprintf("'%s -v -s -noperm -dir %s'", exportCommand, remoteParts[1]))
		} else {
			args = append(args, fmt.Sprintf("'%s -s -noperm -dir %s'", exportCommand, remoteParts[1]))
		}
	}
	sshProc := exec.Command("ssh", args...)
	return sshProc
}

func k8sExport() *exec.Cmd {
	remoteParts := strings.Split(*k8s, ":")
	if len(remoteParts) != 2 {
		fmt.Fprintf(flag.CommandLine.Output(), "Bad pod address: %s", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}

	exportCommand := "/tmp/export9p"
	if *exportPath != "" {
		exportCommand = *exportPath
	} else {
		copyExport9pk8s(remoteParts[0])
	}

	// This shell dance is necessary to pick up the user's profile for PATH and other environment variables.
	args := []string{"exec", "-t", "-i", remoteParts[0], "--", "exec", exportCommand, "-s", "-noperm", "-dir", remoteParts[1]}
	if *loginShell {
		args = []string{"exec", "-t", "-i", remoteParts[0], "--", "exec", "$SHELL", "--login", "-c"}
		if *verbose {
			args = append(args, fmt.Sprintf("'%s -v -s -noperm -dir %s'", exportCommand, remoteParts[1]))
		} else {
			args = append(args, fmt.Sprintf("'%s -s -noperm -dir %s'", exportCommand, remoteParts[1]))
		}
	}
	fmt.Printf("Executing: kubectl %#v\n", args)
	k8sProc := exec.Command("kubectl", args...)
	return k8sProc
}

// TODO(knusbaum): Support copying export9p to ssh targets
// // This only works for linux/amd64 targets right now.
// func copyExport9pSSH() {
// 	cmd := exec.Command("go", "install", "github.com/knusbaum/go9p/cmd/export9p@latest")
// 	cmd.Env = append(os.Environ(), "GOOS=linux")
// 	err := cmd.Run()
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Failed to build export9p for linux: %s\n", err)
// 		os.Exit(2)
// 	}
//
// 	cmd = exec.Command("
// 	$GOPATH/bin/linux_amd64/
// }

// This only works for linux/amd64 targets right now.
func copyExport9pk8s(podname string) {
	cmd := exec.Command("go", "install", "github.com/knusbaum/go9p/cmd/export9p@latest")
	cmd.Env = append(os.Environ(), "GOOS=linux")
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build export9p for linux: %s\n", err)
		os.Exit(2)
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	fmt.Printf("Command: %s %s %s %s \n", "kubectl", "cp", fmt.Sprintf("%s/bin/linux_amd64/export9p", gopath), fmt.Sprintf("%s:/tmp/", podname))
	err = exec.Command("kubectl", "cp", fmt.Sprintf("%s/bin/linux_amd64/export9p", gopath), fmt.Sprintf("%s:/tmp/", podname)).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to copy export9p to pod %s: %s\n", podname, err)
		os.Exit(3)
	}
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] user@address:path localmountpoint\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] -k8s-pod [pod name]:path localmountpoint\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if (*k8s == "" && flag.NArg() < 2) || (*k8s != "" && flag.NArg() < 1) {
		fmt.Printf("NEEDED USAGE\n")
		flag.Usage()
		os.Exit(1)
	}

	var (
		localPath  string
		exportProc *exec.Cmd
	)
	if *k8s != "" {
		localPath = flag.Arg(0)
		exportProc = k8sExport()
	} else {
		localPath = flag.Arg(1)
		exportProc = sshExport()
	}
	exportIn, err := exportProc.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get STDIN: %s", err)
	}
	exportOut, err := exportProc.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get STDOUT: %s", err)
	}
	exportProc.Stderr = os.Stderr

	args := []string{"-dio", "-s", localPath}
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
		exportProc.Start()
		exportProc.Wait()
		done <- struct{}{}
	}()
	defer func() { mountProc.Process.Kill() }()
	defer func() { exportProc.Process.Kill() }()
	<-done
}
