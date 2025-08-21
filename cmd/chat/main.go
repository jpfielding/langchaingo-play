package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var flagModel = flag.String("model", "llama3.2", "model name")
var flagURL = flag.String("url", "http://localhost:11434", "server url")

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

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a company branding design wizard."),
		llms.TextParts(llms.ChatMessageTypeHuman, "What would be a good company name a company that makes colorful socks?"),
	}
	completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	if err != nil {
		log.Fatal(err)
	}
	_ = completion
}
