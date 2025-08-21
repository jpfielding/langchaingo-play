package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jpfielding/gowirelog/wirelog"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var flagModel = flag.String("model", "llama3.2", "model name")
var flagURL = flag.String("url", "http://localhost:11434", "server url")
var flagWirelog = flag.Bool("wirelog", false, "enable wirelog")

func main() {
	flag.Parse()
	transport := wirelog.NewHTTPTransport()
	if *flagWirelog {
		_ = wirelog.LogToWriter(transport, os.Stderr, true, true)
	}
	cl := &http.Client{
		Transport: transport,
	}
	// allow specifying your own model via OLLAMA_MODEL
	// (same as the Ollama unit tests).
	llm, err := ollama.New(
		ollama.WithHTTPClient(cl),
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
