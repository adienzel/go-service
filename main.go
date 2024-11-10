package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gocql/gocql"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Configuration values
type ScyllaConfig struct { // get from env or CLI
	ScyllaDBAddress   string `json:"scyllaDBAddress"`
	KeySpaceName      string `json:"keySpaceName"`
	ReplicationFactor string `json:"replicationFactor"`
	Stategy           string `json:"stategy"`
	TableName         string `json:"tableName"`
}

var (
	serverAddress     string
	numClients        int
	messagesPerSecond float64
	version           string
	loglevel          string
	session           *gocql.Session
	scyllaConfig      ScyllaConfig
)

var Logger *zap.SugaredLogger

const min int = 10000000
const max int = 99999999

func lookupEnvString(env string, defaultParam string) string {
	param, found := os.LookupEnv(env)
	if !found {
		param = defaultParam
	}
	return param
}

func lookupEnvint64(env string, defaultParam int64) int64 {
	param, found := os.LookupEnv(env)
	if !found {
		param = strconv.FormatInt(int64(defaultParam), 10)
	}
	i, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		i = 1
	}
	return i
}

func lookupEnvFloat64(env string, defaultParam float64) float64 {
	param, found := os.LookupEnv(env)
	if !found {
		param = fmt.Sprintf("%f", defaultParam)
	}
	i, err := strconv.ParseFloat(param, 64)
	if err != nil {
		i = 1
	}
	return i
}

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
	server := lookupEnvString("SERVICE_HOST_ADDRES", "127.0.0.1")
	port := lookupEnvString("SERVICE_HOST_PORT", "8992")
	serverAddress = server + ":" + port

	numClients = int(lookupEnvint64("SERVICE_NUMBER_OF_CLIENTS", 1))
	messagesPerSecond = lookupEnvFloat64("SERVICE_MESAGES_PER_SECOND", 1.0)
	loglevel = lookupEnvString("SERVICE_LOGLEVEL", "debug")
	version = lookupEnvString("SERVICE_VERSION", "V1.0")
	scyllaConfig.ScyllaDBAddress = lookupEnvString("SERVICE_SCYLLA_DB_ADDRESS", "127.0.0.1") + ":"
	scyllaConfig.ScyllaDBAddress += lookupEnvString("SERVICE_SCYLLADB_PORT", "9060")
	scyllaConfig.KeySpaceName = lookupEnvString("SERVICE_SCYLLADB_KEYSPACE_NAME", "vin")
	scyllaConfig.ReplicationFactor = lookupEnvString("SERVICE_SCYLLADB_REPLICATION_FACTOR", "1")
	scyllaConfig.Stategy = lookupEnvString("SERVICE_SCYLLADB_STRATEGY", "SimpleStrategy")
	scyllaConfig.TableName = lookupEnvString("SERVICE_SCYLLADB_TABLE_NAME", "vehicles")
}

func (c *ScyllaConfig) createKeyspace() bool {
	q := "CREATE KEYSPACE IF NOT EXISTS " + c.KeySpaceName + " WITH replication = { 'class': '" + c.Stategy +
		"' , 'replication_factor': " + c.ReplicationFactor + "};"
	if err := session.Query(q).Exec(); err != nil {
		Logger.Errorf("failed to execute query", err)
		return false
	}
	return true

}

func (c *ScyllaConfig) createTable() bool {
	q := "CREATE TABLE IF NOT EXISTS" + c.KeySpaceName + "." + c.TableName + " (vin text PRIMARY KEY, host text);"
	if err := session.Query(q).Exec(); err != nil {
		Logger.Errorf("failed to execute query", err)
		return false
	}
	return true
}

func (c *ScyllaConfig) createMatiriaizedView() bool {
	q := "CREATE MATERIALIZED VIEW IF NOT EXISTS " + c.KeySpaceName + "." + c.TableName +
		"_by_host AS SELECT host, vin FROM " + c.KeySpaceName + "." + c.TableName +
		" WHERE host IS NOT NULL AND vin IS NOT NULL PRIMARY KEY (host, vin);"

	if err := session.Query(q).Exec(); err != nil {
		Logger.Errorf("failed to execute query", err)
		return false
	}

	return true
}

func initSession(c *ScyllaConfig) bool {
	cluster := gocql.NewCluster(c.ScyllaDBAddress)

	var err error
	session, err = cluster.CreateSession()
	if err != nil {
		Logger.Errorf("failed to connect ScyllaDB", err)
		return false
	}

	res := c.createKeyspace()
	if !res {
		return res
	}

	res = c.createTable()
	if !res {
		return res
	}

	res = c.createMatiriaizedView()

	return res
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
	fasthttp.ListenAndServe(":9080", externalRequestHandler)
}

func main() {
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

	//TODO build all configuration for the scylla servers
	scyllaConfig := ScyllaConfig{}

	if !initSession(&scyllaConfig) {

	}
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
