package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"github.com/knusbaum/go9p/client"
	"github.com/knusbaum/go9p/proto"
)

func main() {
	s, err := net.Dial("tcp", "localhost:9999")
	if err != nil {
		log.Fatal(err)
	}
	c, err := client.NewClient(s, "kyle", "",
		client.WithAuth(client.PlainAuth("foo")),
	)
	if err != nil {
		log.Fatal(err)
	}
	f, err := c.Open("/dynamic", proto.Oread)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	bs, err := ioutil.ReadAll(f)
	fmt.Printf("RECEIVED: [%s]\n", string(bs))

	fmt.Printf("Reading directory /\n")
	c.Readdir("/")
}
