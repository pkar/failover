/* 
	Failover error log processor for mobage-analytics-proxy

*/
package main

import (
	"client"
	"failover"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"time"
)

const (
	VERSION = "0.5.0"
)

var config = map[string][]string{
	"development": []string{"localhost:15010"},
	"test":        []string{"localhost:15010"},
}

func init() {
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--version" {
			fmt.Printf("%s\n", VERSION)
			os.Exit(0)
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	useSyslog := flag.Bool("syslog", false, "log to syslog")
	applog := flag.String("applog", "", "path to application logging(default STDOUT)")
	errlog := flag.String("errlog", "failover.log", "path to error logged files")
	maxBytes := flag.Int64("maxbytes", 1048576, "Maximum number of bytes before rotating") // Default 1mb
	maxProcessing := flag.Int("maxprocessing", 4, "Maximum number of workers on rotated files")
	env := flag.String("env", "development", "development:testing:staging:sandbox:production")
	debug := flag.Bool("debug", false, "Send actual event or just mock if enabled")

	flag.Parse()

	if *useSyslog {
		sl, err := syslog.New(syslog.LOG_INFO, "failover")
		if err != nil {
			log.Fatalf("Can't initialize syslog: %v", err)
		}
		log.SetOutput(sl)
		defer sl.Close()
	} else if *applog != "" {
		alog, err := os.OpenFile(*applog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err == nil {
			log.SetOutput(alog)
			defer alog.Close()
		}
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetPrefix("[mobage-analytics-proxy-failover] ")

	quit := make(chan struct{})
	done := make(chan string)

	conns, ok := config[*env]
	clients := make([]*client.Client, 0, len(conns))

	if ok {
		for _, host := range conns {
			c := client.NewClient(host)
			if c != nil {
				clients = append(clients, c)
			}
		}
	}

	fa := &failover.Failover{
		Debug:         *debug,
		Env:           *env,
		MaxBytes:      *maxBytes,
		MaxProcessing: *maxProcessing,
		Errlog:        *errlog,
		Clients:       clients,
		Quit:          quit,
		Done:          done,
	}

	go fa.FileRotator()
	go fa.FileWatcher()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	exit := false
	for !exit {
		log.Printf("Running failover %s in %s %s", VERSION, *env, *errlog)
		select {
		case sig := <-interrupt:
			log.Printf("Captured %v, exiting..", sig)
			fa.Close()
			exit = true
			break
		}
	}
}
