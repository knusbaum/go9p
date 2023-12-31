package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func unusedUid() uint32 {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return 0
	}
	highest := 0
	defer f.Close()
	rd := bufio.NewReader(f)
	for l, _, err := rd.ReadLine(); err == nil; l, _, err = rd.ReadLine() {
		ss := strings.Split(string(l), ":")
		uid, err := strconv.Atoi(ss[2])
		if err != nil {
			return 0
		}
		if uid > highest {
			highest = uid
		}
	}
	return uint32(highest + 1)
}

func unusedGid() uint32 {
	f, err := os.Open("/etc/group")
	if err != nil {
		return 0
	}
	highest := 0
	defer f.Close()
	rd := bufio.NewReader(f)
	for l, _, err := rd.ReadLine(); err == nil; l, _, err = rd.ReadLine() {
		ss := strings.Split(string(l), ":")
		uid, err := strconv.Atoi(ss[2])
		if err != nil {
			return 0
		}
		if uid > highest {
			highest = uid
		}
	}
	return uint32(highest + 1)
}
