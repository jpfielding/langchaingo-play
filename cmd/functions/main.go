package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"

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

		if c := unmarshalCall(choice1.Content); c != nil {
			log.Printf("Call: %v", c.Tool)
			if *flagVerbose {
				log.Printf("Call: %v (raw: %v)", c.Tool, choice1.Content)
			}
			msg, cont := dispatchCall(c)
			if !cont {
				break
			}
			msgs = append(msgs, msg)
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

type Call struct {
	Tool     string         `json:"tool"`
	Input    map[string]any `json:"tool_input"`
	Response string         `json:"response"`
}

func unmarshalCall(input string) *Call {
	var c Call
	if err := json.Unmarshal([]byte(input), &c); err == nil && c.Tool != "" {
		return &c
	}
	return nil
}

func dispatchCall(c *Call) (llms.MessageContent, bool) {
	// ollama doesn't always respond with a *valid* function call. As we're using prompt
	// engineering to inject the tools, it may hallucinate.
	if !validTool(c.Tool) {
		log.Printf("invalid function call: %#v, prompting model to try again", c)
		return llms.TextParts(llms.ChatMessageTypeHuman,
			"Tool does not exist, please try again."), true
	}

	// we could make this more dynamic, by parsing the function schema.
	switch c.Tool {
	case "getCurrentWeather":
		loc, ok := c.Input["location"].(string)
		if !ok {
			log.Fatal("invalid input")
		}
		unit, ok := c.Input["unit"].(string)
		if !ok {
			log.Fatal("invalid input")
		}

		weather, err := getCurrentWeather(loc, unit)
		if err != nil {
			log.Fatal(err)
		}
		return llms.TextParts(llms.ChatMessageTypeHuman, weather), true
	case "finalResponse":
		// resp, ok := c.Input["response"].(string)
		// if !ok {
		// 	log.Fatal("invalid input")
		// }
		resp := c.Response
		log.Printf("Final response: %v", resp)

		return llms.MessageContent{}, false
	default:
		// we already checked above if we had a valid tool.
		panic("unreachable")
	}
}

func validTool(name string) bool {
	var valid []string
	for _, v := range functions {
		valid = append(valid, v.Name)
	}
	return slices.Contains(valid, name)
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
func getCurrentWeather(location string, unit string) (string, error) {
	apiKey := os.Getenv("WEATHER_API_KEY")
	uri := fmt.Sprintf("http://api.weatherapi.com/v1/forecast.json?key=%s&q=%s&days=1&api=no&alerts=no", apiKey, location)
	res, err := http.Get(uri)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		panic("Weather API not available")
	}
	var weather Weather
	if err = json.NewDecoder(res.Body).Decode(&weather); err != nil {
		panic(err)
	}
	weatherInfo := map[string]any{
		"location":    weather.Location.Name,
		"temperature": weather.Current.TempC,
		"unit":        "celsius",
		"forecast":    []string{weather.Current.Condition.Text},
	}
	b, err := json.Marshal(weatherInfo)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func GetDummyWeather(location string, unit string) (string, error) {
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
		Name:        "getCurrentWeather",
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

type Weather struct {
	Location struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"location"`

	Current struct {
		TempC     float64 `json:"temp_c"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
	} `json:"current"`

	Forecast struct {
		ForecastDay []struct {
			Hour []struct {
				TimeEpoch int64   `json:"time_epoch"`
				TempC     float64 `json:"temp_c"`
				Condition struct {
					Text string `json:"text"`
				} `json:"condition"`
				ChanceOfRain float64 `json:"chance_of_rain"`
			} `json:"hour"`
		} `json:"forecastday"`
	} `json:"forecast"`
}
