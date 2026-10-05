// Run with SAYMAKER_API_KEY set: go run ./example "a paper boat at sunrise"
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hanshs474/saymaker-go"
)

func main() {
	prompt := "a paper-cut layered mountain range at dusk, warm rim light"
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}
	url, err := saymaker.Generate(context.Background(), prompt, saymaker.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(url)
}
