package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	disCol       = true
	exporterName = "atlassian_instance_health"
	url          string

	address       = flag.String("svc.address", "0.0.0.0", "assign an IP address for this service to listen on")
	debug         = flag.Bool("debug", false, "enable the service debug output")
	enableColLogs = flag.Bool("enable-color-logs", false, "when developing in debug mode, prettier to set this for visual colors")
	fqdn          = flag.String("app.fqdn", "", "REQUIRED: set the fqdn of the application (ie. <jira|confluence>.domain.com)")
	help          = flag.Bool("help", false, "pass help will display this helpful dialog output.")
	port          = flag.String("svc.port", "9998", "set the port that this service will listen on")
	protocal      = flag.String("app.protocal", "https", "set the protocal for the application. [http|https]")
	token         = flag.String("app.token", "", "REQUIRED: set the basic token for the service to make requests as")

	usageMessage = "The Atlassin Instance Health Exporter is used in conjunction with the Atlassian\n" +
		"Troubleshooting and Support Tools Plugin. The Instance Health feature is currently available\n" +
		"for Confluence and Jira. The application account that this container will use to reach\n" +
		"out and scrape that endpoint will need to have Administrator access. Once the plugin is\n" +
		"installed and the account it setup, you can run the exporter against the endpoint and\n" +
		"this container will turn the endpoint into metrics.\n" +
		"\nReference:\n" +
		"https://confluence.atlassian.com/support/instance-health-790796828.html\n" +
		"\nUsage: " + exporterName + " [Arguments...]\n" +
		"\nArguments:"
)

// Instance Health structure associated with the endpoint.
type instanceHealthEndpoint struct {
	Statuses []struct {
		ID            int    `json:"id"`
		CompleteKey   string `json:"completeKey"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		IsHealthy     bool   `json:"isHealthy"`
		FailureReason string `json:"failureReason"`
		Application   string `json:"application"`
		Time          int64  `json:"time"`
		Severity      string `json:"severity"`
		Documentation string `json:"documentation"`
		Tag           string `json:"tag"`
		Healthy       bool   `json:"healthy"`
	} `json:"statuses"`
}

// usage is a function used to display this binaries usage.
var usage = func() {
	fmt.Println(usageMessage)
	flag.PrintDefaults()
	os.Exit(0)
}

// instanceHealthCollector is the structure of our prometheus collector containing it descriptors.
type instanceHealthCollector struct {
	instanceHealthMetric        *prometheus.Desc
	instanceHealthRuntimeMetric *prometheus.Desc
	instanceHealthUpMetric      *prometheus.Desc
}

// newInstanceHealthCollector is the constructor for our collector used to initialize the metrics.
func newInstanceHealthCollector() *instanceHealthCollector {
	return &instanceHealthCollector{
		instanceHealthMetric: prometheus.NewDesc(
			exporterName,
			"metric used to monitor the Atlassian Troubleshooting and Support Tools Plugin endpoint (https://<url>/rest/troubleshooting/1.0/check/)",
			[]string{
				"id",
				"completekey",
				"name",
				"description",
				"failurereason",
				"application",
				"severity",
				"documentation",
				"tag",
				"fqdn",
			},
			nil,
		),
		instanceHealthRuntimeMetric: prometheus.NewDesc(
			exporterName+"_collect_duration_seconds",
			"Used to keep track of how long the exporter took to collect metrics",
			[]string{
				"fqdn",
			},
			nil,
		),
		instanceHealthUpMetric: prometheus.NewDesc(
			exporterName+"_scrape_url_up",
			"metric used to check if the rest endpoint is accessible (https://<url>/rest/troubleshooting/1.0/check/)",
			[]string{
				"httpcode",
				"fqdn",
			},
			nil,
		),
	}
}

// Describe is required by prometheus to add our metrics to the default prometheus desc channel
func (collector *instanceHealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.instanceHealthMetric
	ch <- collector.instanceHealthRuntimeMetric
	ch <- collector.instanceHealthUpMetric
}

// Collect implements required collect function for all prometheus collectors
func (collector *instanceHealthCollector) Collect(ch chan<- prometheus.Metric) {

	startTime := time.Now()

	log.Debug("create a request object")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("http.NewRequest returned an error:", err)
	}

	log.Debug("create a basic auth string from argument passed")
	basic := "Basic " + *token

	log.Debug("add authorization header to the request")
	req.Header.Add("Authorization", basic)

	log.Debug("set content type on the request")
	req.Header.Add("content-type", "application/json")

	log.Debug("get url: ", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn("http.Get base URL returned an error:", err)
		ch <- prometheus.MustNewConstMetric(collector.instanceHealthUpMetric, prometheus.GaugeValue, 0, "", *fqdn)
		return
	}
	defer resp.Body.Close()

	log.Debug("set scrape metric statuscode: ", strconv.Itoa(resp.StatusCode))
	ch <- prometheus.MustNewConstMetric(collector.instanceHealthUpMetric, prometheus.GaugeValue, 1, strconv.Itoa(resp.StatusCode), *fqdn)

	log.Debug("get the body out of the response")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("ioutil.ReadAll returned an error:", err)
	}

	log.Debug("turn the response body into a map")
	m := instanceHealth(body)
	log.Debug("the returned body map: ", m)

	// range over the map to create each metric with it's labels.
	for _, metric := range m.Statuses {
		log.Debug("create healthcode metric for: ", metric.Description)
		ch <- prometheus.MustNewConstMetric(
			collector.instanceHealthMetric,
			prometheus.GaugeValue,
			boolToFloat(metric.IsHealthy),
			strconv.Itoa(metric.ID),
			metric.CompleteKey,
			metric.Name,
			metric.Description,
			metric.FailureReason,
			metric.Application,
			metric.Severity,
			metric.Documentation,
			metric.Tag,
			*fqdn,
		)
	}

	finishTime := time.Now()
	elapsedTime := finishTime.Sub(startTime)
	log.Debug("set the duration metric")
	ch <- prometheus.MustNewConstMetric(collector.instanceHealthRuntimeMetric, prometheus.GaugeValue, elapsedTime.Seconds(), *fqdn)
	log.Debug("collect finished")
}

// instanceHealth takes a http body btye slice and unmarshals it into the /rest/troubleshooting/1.0/check/ structure.
func instanceHealth(body []byte) instanceHealthEndpoint {

	log.Debug("create the json map to unmarshal the json body into")
	var m instanceHealthEndpoint

	log.Debug("unmarshal (turn unicode back into a string) request body into map structure")
	err := json.Unmarshal(body, &m)
	if err != nil {
		log.Error("error Unmarshalling: ", err)
		log.Info("Problem unmarshalling the following string: ", string(body))
	}

	return m
}

// rootHandler accepts calls to "/". This can be used to see if the service is running.
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, exporterName+" is running")
	log.Info(r.RemoteAddr, " requested ", r.URL)
}

// faviconHandler responds to /favicon.ico requests.
// This is set to stop error logs from generating when certian browsers that request favicon.ico and the server doesn't have that page.
func faviconHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "")
}

// boolToFloat converts a boolean value to a float64
func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	flag.Parse()

	// Check if help has been passed
	if *help {
		usage()
	}

	// check for required arguments
	if *token == "" {
		fmt.Printf("app.token needs to be set.\n\n")
		usage()
	}
	if *fqdn == "" {
		fmt.Printf("app.fqdn needs to be set.\n\n")
		usage()
	}

	// adjust the logrus logger. Disable colors by default (adjustable with enable-color-logs option). Enable full time-stamps by default
	if *enableColLogs {
		disCol = false
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		DisableColors: disCol,
	})

	// check for debug option, adjust if set
	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Log Level: debug")
	}

	// Create a new instance of the Collector and then
	// register it with the prometheus client.
	exporter := newInstanceHealthCollector()
	prometheus.MustRegister(exporter)

	log.Info("starting...")

	log.Debug("create http server listening at: ", *address, ":", *port)
	srv := http.Server{
		Addr: *address + ":" + *port,
	}

	log.Debug("add handlers to http server")
	log.Debug("add / handler")
	http.HandleFunc("/", rootHandler)

	log.Debug("add /favicon.ico handler") // because browsers request /favicon.ico, we add a handler so our metrics don't get false calls
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Debug("add /metrics handler")
	http.Handle("/metrics", promhttp.Handler())

	url = *protocal + "://" + *fqdn + "/rest/troubleshooting/1.0/check/"
	log.Debug("set the endpoint url to: ", url)

	log.Debug("make a channel of type os.Signal with a 1 space buffer size")
	ch := make(chan os.Signal, 1)

	// when a SIGNAL of a certain type happens, put it 'on' the channel
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	log.Debug("start the http server in a goroutine (pew -->)")
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal("ListenAndServe Error:", err)
		}
	}()

	log.Info(exporterName, " is ready to take requests at: ", *address+":"+*port)

	// channels block, so the program will wait (stay running) here till it gets a signal
	s := <-ch
	log.Info("SIGNAL received: ", s)

	close(ch)
	log.Debug("signal channel closed")

	log.Info("shutting down http server...")
	err := srv.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout
		log.Fatal("Shutdown error: ", err)
	}

	log.Info(exporterName, " was gracefully shutdown")
}
