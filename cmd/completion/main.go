package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var flagVerbose = flag.Bool("v", false, "verbose mode")
var flagModel = flag.String("model", "llama3.2", "model name")
var flagURL = flag.String("url", "localhost:11434", "server url")

func main() {
	flag.Parse()
	// allow specifying your own model via OLLAMA_MODEL
	// (same as the Ollama unit tests).
	llm, err := ollama.New(
		ollama.WithServerURL(*flagURL),
		ollama.WithModel(*flagModel),
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	completion, err := llms.GenerateFromSinglePrompt(
		ctx,
		llm,
		"Human: Who was the first man to walk on the moon?\nAssistant:",
		llms.WithTemperature(0.8),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = completion
}
