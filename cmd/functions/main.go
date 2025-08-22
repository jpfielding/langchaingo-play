package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jpfielding/gowirelog/wirelog"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var flagVerbose = flag.Bool("v", false, "verbose mode")
var flagModel = flag.String("model", "llama3.2", "model name")
var flagURL = flag.String("url", "http://localhost:11434", "server url")
var flagWirelog = flag.Bool("wirelog", false, "enable wirelog")
var flagLocation = flag.String("location", "Beijing", "location for weather query")

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

	var msgs []llms.MessageContent

	// system message defines the available tools.
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeSystem, systemMessage()))
	msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, fmt.Sprintf("What's the weather like in %s?", *flagLocation)))

	ctx := context.Background()

	for retries := 3; retries > 0; retries = retries - 1 {
		resp, err := llm.GenerateContent(ctx, msgs)
		if err != nil {
			log.Fatal(err)
		}

		choice1 := resp.Choices[0]
		msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeAI, choice1.Content))
		call := Call{}
		final := Response{} // this shouldnt be necessary, but Ollama doesn't always respond with a function call.

		if err = json.Unmarshal([]byte(choice1.Content), &call); err == nil && call.Tool != "" {
			log.Printf("Call: %v", call.Tool)
			if *flagVerbose {
				log.Printf("Call: %v (raw: %v)", call.Tool, choice1.Content)
			}
			if call.Tool == "finalResponse" {
				log.Printf("Final response: %v", call.Response)
				return
			}
			msg := dispatchCall(call)
			msgs = append(msgs, msg)
		} else if err = json.Unmarshal([]byte(choice1.Content), &final); err == nil && final.Final != "" {
			log.Printf("Final response: %v", final.Final)
			return
		} else {
			// Ollama doesn't always respond with a function call, let it try again.
			log.Printf("Not a call: %v", choice1.Content)
			msgs = append(msgs, llms.TextParts(llms.ChatMessageTypeHuman, "Sorry, I don't understand. Please try again."))
		}

		if retries == 0 {
			log.Fatal("retries exhausted")
		}
	}
}

type Response struct {
	Final string `json:"finalResponse"`
}

type Call struct {
	Tool     string         `json:"tool"`
	Input    map[string]any `json:"tool_input"`
	Response string         `json:"response"`
}

func dispatchCall(c Call) llms.MessageContent {
	// we could make this more dynamic, by parsing the function schema.
	switch c.Tool {
	case "get_current_weather":
		loc, ok := c.Input["location"].(string)
		if !ok {
			log.Fatal("invalid input")
		}
		unit, ok := c.Input["unit"].(string)
		if !ok {
			log.Fatal("invalid input")
		}
		weather, err := get_current_weather(loc, unit)
		if err != nil {
			panic(err)
		}
		return llms.TextParts(llms.ChatMessageTypeHuman, weather)
	default:
		return llms.TextParts(
			llms.ChatMessageTypeHuman,
			"Tool does not exist, please try again.",
		)
	}
}

func systemMessage() string {
	bs, err := json.Marshal(functions)
	if err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf(`You have access to the following tools:

%s

To use a tool, respond with a JSON object with the following structure: 
{
	"tool": <name of the called tool>,
	"tool_input": <parameters for the tool matching the above JSON schema>
}
`, string(bs))
}

func get_current_weather(location string, unit string) (string, error) {
	weatherInfo := map[string]any{
		"location":    location,
		"temperature": "6",
		"unit":        unit,
		"forecast":    []string{"sunny", "windy"},
	}
	if unit == "fahrenheit" {
		weatherInfo["temperature"] = 43
	}

	b, err := json.Marshal(weatherInfo)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var functions = []llms.FunctionDefinition{
	{
		Name:        "get_current_weather",
		Description: "Get the current weather in a given location",
		Parameters: json.RawMessage(`{
			"type": "object", 
			"properties": {
				"location": {"type": "string", "description": "The city and state, e.g. San Francisco, CA"}, 
				"unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
			}, 
			"required": ["location", "unit"]
		}`),
	},
	{
		// I found that providing a tool for Ollama to give the final response significantly
		// increases the chances of success.
		Name:        "finalResponse",
		Description: "Provide the final response to the user query",
		Parameters: json.RawMessage(`{
			"type": "object", 
			"properties": {
				"response": {"type": "string", "description": "The final response to the user query"}
			}, 
			"required": ["response"]
		}`),
	},
}
