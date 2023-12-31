package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/knusbaum/go9p/cert"
)

func exists(file string) bool {
	_, err := os.Stat(file)
	return !errors.Is(err, os.ErrNotExist)
}

func main() {

	authority := flag.String("authority", "", "The authority file to use. One will be generated if not present.")
	certfile := flag.String("certfile", "", "The PEM-encoded certificate and private key for a server or client")
	user := flag.String("user", "", "The username associated with the certificate. This will allow someone with this certificate to login as this user.")

	flag.Parse()

	if *authority == "" {
		fmt.Printf("Error: Need an authority file.\n")
		flag.PrintDefaults()
		return
	}

	ca, pk, err := cert.LoadCA(*authority)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cert.GenCA(*authority)
			ca, pk, err = cert.LoadCA(*authority)
			if err != nil {
				fmt.Printf("Failed to generate authority file: %v\n", err)
				return
			}
		} else {
			fmt.Printf("Failed to load authority file: %v\n", err)
			return
		}
	}

	if *certfile != "" {
		if exists(*certfile) {
			fmt.Printf("%s already exists. Not overwriting.\n", *certfile)
			return
		}
		if *user == "" {
			fmt.Printf("Must specify a user name to generate a cert.\n")
			return
		}

		err := cert.GenCert(*user, *certfile, ca, pk)
		if err != nil {
			fmt.Printf("Failed to generate certificate: %s\n", err)
			return
		}
	}
}
