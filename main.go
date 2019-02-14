package main

import (
	"github.com/haposan06/golang-blockchain/cli"
	"os"
)



func main() {
	defer os.Exit(0)
	cl := cli.CommandLine{}
	cl.Run()
}