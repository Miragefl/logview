package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: logview <command> [args]")
		fmt.Println("Commands: k8s, tail, pipe")
		os.Exit(1)
	}
	fmt.Printf("logview: unknown command %q\n", os.Args[1])
	os.Exit(1)
}