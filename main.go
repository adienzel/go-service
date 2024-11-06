package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"flag"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Configuration values
var (
	serverAddress     string
	numClients        int
	messagesPerSecond float64
	version           string
	loglevel          string
)

var Logger *zap.SugaredLogger

const min int = 10000000
const max int = 99999999

func getLevelLogger(loglevel string) zapcore.Level {
	switch {
	case loglevel == "debug":
		return zap.DebugLevel
	case loglevel == "info":
		return zap.InfoLevel
	case loglevel == "warning":
		return zap.WarnLevel
	case loglevel == "error":
		return zap.ErrorLevel
	default:
		return zap.DebugLevel
	}
}

func init() {
	// CLI flags for the application
	flag.StringVar(&serverAddress, "server", "http://localhost:8080", "Server address and port")
	flag.IntVar(&numClients, "clients", 1, "Number of WebSocket clients")
	flag.Float64Var(&messagesPerSecond, "rate", 1.0, "Rate of messages per second (float)")
	flag.StringVar(&loglevel, "Loglevel", "debug", "Loglevel can be  [error, warning, info, debug] defasult is debug")
	flag.StringVar(&version, "version", "v1", "Version of the request")
	flag.Parse()
}

// Request structure to be sent in both URL and JSON body
type RequestBody struct {
	Version     string `json:"version"`
	VIN         string `json:"VIN"`
	Command     string `json:"command"`
	Seconds     int64  `json:"seconds"`
	Nanoseconds int64  `json:"nanoseconds"`
}

type ResponseBody struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Seconds     int64  `json:"seconds"`
	Nanoseconds int64  `json:"nanoseconds"`
}

// Function to send a POST request with the version, VIN, and command using fasthttp
func sendRequest(clientID int, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	rand.New(rand.NewSource(int64(clientID))) // maintain the same client id and it will be the same between stoping and stating app
	vin := rand.Intn(max-min+1) + min
	client := &fasthttp.Client{}
	ticker := time.NewTicker(time.Second / time.Duration(messagesPerSecond))
	defer ticker.Stop()

	for {
		// Wait for ticker to send requests at defined rate
		<-ticker.C

		// Generate version, VIN, and command for the request

		url := fmt.Sprintf("%s/%s/%d/%s", serverAddress, version, vin, "openDoor")

		// Create timestamp for the request
		timestamp := time.Now()

		// Prepare the request body as JSON, including the timestamp
		body := RequestBody{
			Version:     version,
			VIN:         strconv.FormatInt(int64(vin), 10),
			Command:     "openDoor",
			Seconds:     int64(timestamp.Second()),
			Nanoseconds: int64(timestamp.Nanosecond()),
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			logger.Errorf("Client %d: Error marshalling JSON: %v", clientID, err)
			return
		}

		// Create the fasthttp request
		req := fasthttp.AcquireRequest()
		req.SetRequestURI(url)
		req.Header.Set("Content-Type", "application/json")
		req.SetBody(jsonBody)

		// Create the fasthttp response
		resp := fasthttp.AcquireResponse()

		// Send the request
		err = client.Do(req, resp)
		if err != nil {
			logger.Errorf("Client %d: Error sending request: %v", clientID, err)
			return
		}

		// Parse the response body
		var responseBody ResponseBody
		if err := json.Unmarshal(resp.Body(), &responseBody); err != nil {
			logger.Errorf("Client %d: Error unmarshalling response: %v", clientID, err)
			return
		}

		// Log the server response
		logger.Infof("Client %d: Sent request to %s, received status %d, timestamp %d.%d", clientID, url, resp.StatusCode(), responseBody.Seconds, responseBody.Nanoseconds)
	}
}

// External HTTP server handler (to simulate receiving requests)
func externalRequestHandler(ctx *fasthttp.RequestCtx) {
	// Parse URL parameters
	version := ctx.UserValue("version").(string)
	vins := ctx.UserValue("vin").(string)
	command := ctx.UserValue("command").(string)

	// Read the JSON body
	var requestBody RequestBody
	if err := json.Unmarshal(ctx.PostBody(), &requestBody); err != nil {
		Logger.Errorf("Invalid JSON body, ")
		ctx.Error("Invalid JSON body", fasthttp.StatusBadRequest)
		return
	}

	// Log received request
	Logger.Infof("Received request: version=%s, VIN=%s, command=%s, timestamp=%d.%d", version, vins, command, requestBody.Seconds, requestBody.Nanoseconds)
	Logger.Infof("Received JSON body: %+v", requestBody)

	// Add a timestamp to the response body (the server's processing time)
	timestamp := time.Now()
	responseBody := ResponseBody{
		Status:      "ok",
		Message:     "Request received successfully",
		Seconds:     int64(timestamp.Second()),
		Nanoseconds: int64(timestamp.Nanosecond()),
	}

	// Send a response back
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	respJSON, _ := json.Marshal(responseBody)
	ctx.SetBody(respJSON)
}

// Start the external HTTP server to handle incoming requests using fasthttp
func startExternalServer() {
	// Use fasthttp request handler
	fasthttp.ListenAndServe(":8081", externalRequestHandler)
}

func main() {
	var err error
	level := zap.NewAtomicLevelAt(getLevelLogger(loglevel))
	encoder := zap.NewProductionEncoderConfig()

	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig = encoder
	zapConfig.Level = level
	// zapConfig.Development = config.IS_DEVELOP_MODE
	zapConfig.Encoding = "json"
	//zapConfig.InitialFields = map[string]interface{}{"idtx": "999"}
	zapConfig.OutputPaths = []string{"stdout"} // can add later a log file
	zapConfig.ErrorOutputPaths = []string{"stderr"}
	logger, err := zapConfig.Build()

	if err != nil {
		panic(err)
	}
	Logger = logger.Sugar()
	// Start the external HTTP server in a separate Goroutine
	go startExternalServer()

	Logger.Infof("Starting client application: Server=%s, Clients=%d, Rate=%.2f msgs/sec", serverAddress, numClients, messagesPerSecond)

	var wg sync.WaitGroup

	// Start multiple clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go sendRequest(i+1, &wg, Logger)
	}

	// Wait for all clients to finish
	wg.Wait()
}
