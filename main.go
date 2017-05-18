package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/GaryBoone/GoStats/stats"
	"math"
	"runtime"
)

// Message describes a message
type Message struct {
	Topic     string
	QoS       byte
	Payload   interface{}
	Sent      time.Time
	Delivered time.Time
	Error     bool
}

// RunResults describes results of a single client / run
type RunResults struct {
	ID          int     `json:"id"`
	Successes   int64   `json:"successes"`
	Failures    int64   `json:"failures"`
	Total       int64   `json:"total"`
	RunTime     float64 `json:"run_time"`
	MsgTimeMin  float64 `json:"msg_time_min"`
	MsgTimeMax  float64 `json:"msg_time_max"`
	MsgTimeMean float64 `json:"msg_time_mean"`
	MsgTimeStd  float64 `json:"msg_time_std"`
	MsgsPerSec  float64 `json:"msgs_per_sec"`
}

// TotalResults describes results of all clients / runs
type TotalResults struct {
	Ratio           float64 `json:"ratio"`
	Successes       int64   `json:"successes"`
	Failures        int64   `json:"failures"`
	Total       	int64   `json:"total"`
	TotalRunTime    float64 `json:"total_run_time"`
	AvgRunTime      float64 `json:"avg_run_time"`
	MsgTimeMin      float64 `json:"msg_time_min"`
	MsgTimeMax      float64 `json:"msg_time_max"`
	MsgTimeMeanAvg  float64 `json:"msg_time_mean_avg"`
	MsgTimeMeanStd  float64 `json:"msg_time_mean_std"`
	TotalMsgsPerSec float64 `json:"total_msgs_per_sec"`
	AvgMsgsPerSec   float64 `json:"avg_msgs_per_sec"`
}

// JSONResults are used to export results as a JSON document
type JSONResults struct {
	Runs   []*RunResults `json:"runs"`
	Totals *TotalResults `json:"totals"`
}

func main() {

	var (
		broker   = flag.String("broker", "tcp://localhost:1883", "MQTT broker endpoint as scheme://host:port")
		topic    = flag.String("topic", "/test", "MQTT topic for outgoing messages")
		username = flag.String("username", "", "MQTT username (empty if auth disabled)")
		password = flag.String("password", "", "MQTT password (empty if auth disabled)")
		qos      = flag.Int("qos", 1, "QoS for published messages")
		size     = flag.Int("size", 100, "Size of the messages payload (bytes)")
		count    = flag.Int("count", 100, "Number of messages to send per client")
		msgtimeout  = flag.Int("msgtimeout", 50, "Timeout (ms) when send message to broker")
		msgdelay    = flag.Int("msgdelay", 0, "Delay (ms) between send each message")
		delay    = flag.Int("delay", 50, "Delay (ms) between start each client")
		clients  = flag.Int("clients", 10, "Number of clients to start")
		format   = flag.String("format", "text", "Output format: text|json")
		quiet    = flag.Bool("quiet", false, "Suppress logs while running")
		cpu  = flag.Int("cpu", 1, "Number of cpu to use")
	)
	// set number of thread to use
	runtime.GOMAXPROCS(*cpu)

	flag.Parse()
	if *clients < 1 {
		log.Fatal("Invlalid arguments")
	}

	resCh := make(chan *RunResults)
	start := time.Now()
	for i := 0; i < *clients; i++ {
		if !*quiet {
			log.Println("Starting client ", i)
		}
		c := &Client{
			ID:         i,
			BrokerURL:  *broker,
			BrokerUser: *username,
			BrokerPass: *password,
			MsgTopic:   *topic,
			MsgSize:    *size,
			MsgCount:   *count,
			MsgTimeOut: *msgtimeout,
			MsgDelay:	*msgdelay,
			MsgQoS:     byte(*qos),
			Quiet:      *quiet,
		}
		go c.Run(resCh)
		time.Sleep(time.Duration(*delay)*time.Millisecond)
	}

	// collect the results
	results := make([]*RunResults, *clients)
	for i := 0; i < *clients; i++ {
		results[i] = <-resCh
		log.Printf("CLIENT %v is completed (%d/%d)\n", results[i].ID, i+1, *clients)
	}
	totalTime := time.Now().Sub(start)
	totals := calculateTotalResults(results, totalTime)

	// print stats
	printResults(results, totals, *format)
}
func calculateTotalResults(results []*RunResults, totalTime time.Duration) *TotalResults {
	totals := new(TotalResults)
	totals.TotalRunTime = totalTime.Seconds()

	msgTimeMeans := make([]float64, len(results))
	msgsPerSecs := make([]float64, len(results))
	runTimes := make([]float64, len(results))
	bws := make([]float64, len(results))

	totals.MsgTimeMin = results[0].MsgTimeMin
	for i, res := range results {
		totals.Successes += res.Successes
		totals.Failures += res.Failures
		totals.Total += res.Total
		totals.TotalMsgsPerSec += res.MsgsPerSec

		if res.Successes>0 {
			if math.IsNaN(res.MsgTimeMin) == false && (math.IsNaN(totals.MsgTimeMin) == true || res.MsgTimeMin < totals.MsgTimeMin) {
				totals.MsgTimeMin = res.MsgTimeMin
			}

			if math.IsNaN(res.MsgTimeMax) == false && (math.IsNaN(totals.MsgTimeMax) == true || res.MsgTimeMax > totals.MsgTimeMax) {
				totals.MsgTimeMax = res.MsgTimeMax
			}

			msgTimeMeans[i] = res.MsgTimeMean
			msgsPerSecs[i] = res.MsgsPerSec
			runTimes[i] = res.RunTime
			bws[i] = res.MsgsPerSec
		}
	}

	totals.Ratio = float64(totals.Successes) / float64(totals.Total)
	totals.AvgMsgsPerSec = stats.StatsMean(msgsPerSecs)
	totals.AvgRunTime = stats.StatsMean(runTimes)
	totals.MsgTimeMeanAvg = stats.StatsMean(msgTimeMeans)
	totals.MsgTimeMeanStd = stats.StatsSampleStandardDeviation(msgTimeMeans)

	return totals
}

func printResults(results []*RunResults, totals *TotalResults, format string) {
	switch format {
	case "json":
		jr := JSONResults{
			Runs:   results,
			Totals: totals,
		}
		data, _ := json.Marshal(jr)
		var out bytes.Buffer
		json.Indent(&out, data, "", "\t")

		fmt.Println(string(out.Bytes()))
	default:
		var failedClient = 0
		for _, res := range results {
			if res.Successes==0 {
				fmt.Printf("======= FAILED CLIENT %d =======\n", res.ID)
				//fmt.Printf("Ratio:               %.3f (%d+%d/%d)\n", float64(res.Successes)/float64(res.Successes+res.Failures), res.Successes, res.Failures, res.Total)
				//fmt.Printf("Runtime (s):         %.3f\n", res.RunTime)
				//fmt.Printf("Msg time min (ms):   %.3f\n", res.MsgTimeMin)
				//fmt.Printf("Msg time max (ms):   %.3f\n", res.MsgTimeMax)
				//fmt.Printf("Msg time mean (ms):  %.3f\n", res.MsgTimeMean)
				//fmt.Printf("Msg time std (ms):   %.3f\n", res.MsgTimeStd)
				//fmt.Printf("Bandwidth (msg/sec): %.3f\n\n", res.MsgsPerSec)
				failedClient++;
			}
		}
		fmt.Printf("========= TOTAL (%d/%d) =========\n", len(results)-failedClient, len(results))
		fmt.Printf("Total Ratio:                 %.3f (%d/%d)\n", totals.Ratio, totals.Successes, totals.Total)
		fmt.Printf("Total Runtime (sec):         %.3f\n", totals.TotalRunTime)
		fmt.Printf("Average Runtime (sec):       %.3f\n", totals.AvgRunTime)
		fmt.Printf("Msg time min (ms):           %.3f\n", totals.MsgTimeMin)
		fmt.Printf("Msg time max (ms):           %.3f\n", totals.MsgTimeMax)
		fmt.Printf("Msg time mean mean (ms):     %.3f\n", totals.MsgTimeMeanAvg)
		fmt.Printf("Msg time mean std (ms):      %.3f\n", totals.MsgTimeMeanStd)
		fmt.Printf("Average Bandwidth (msg/sec): %.3f\n", totals.AvgMsgsPerSec)
		fmt.Printf("Total Bandwidth (msg/sec):   %.3f\n", totals.TotalMsgsPerSec)
	}
	return
}
