package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jpfielding/gowirelog/wirelog"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/tools/serpapi"
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
	// Initialize the SerpAPI tool for web searches
	search, err := serpapi.New(serpapi.WithAPIKey(os.Getenv("SERPAPI_API_KEY")))
	if err != nil {
		panic(err)
	}

	// Define the tools the agent can use
	agentTools := []tools.Tool{
		tools.Calculator{},
		search,
	}

	// Create a new ReAct agent with the Ollama LLM and tools
	agent := agents.NewOneShotAgent(llm, agentTools, agents.WithMaxIterations(3))

	// Create the executor to run the agent
	executor := agents.NewExecutor(agent)

	// Run the agent with the same complex query
	input := "Who is Olivia Wilde's boyfriend? What is his current age raised to the 0.23 power?"
	result, err := executor.Call(ctx, map[string]any{"input": input})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Result: %s\n", result["output"])
}
