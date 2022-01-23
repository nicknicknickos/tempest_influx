package main

import (
	"flag"
	"log"
	"net"
	"net/url"
	"net/http"
	"os"
	"strings"
)

var opts struct {
	Source string
	Target string
	Token string
	Bucket string
	Buffer int
	Verbose bool
	Debug bool
}

func main() {
	logger := log.New(os.Stdout, "tempest_influx: ", log.LstdFlags)

	flag.StringVar(&opts.Source, "source", ":50222", "Source port to listen on")
	flag.StringVar(&opts.Target, "target", "https://localhost:50222/api/v2/write", "URL to receive influx metrics")
	flag.StringVar(&opts.Token, "token", "", "Authentication token")
	flag.StringVar(&opts.Bucket, "bucket", "", "InfluxDB bucket name")
	flag.IntVar(&opts.Buffer, "buffer", 10240, "Max buffer size for the socket io")
	flag.BoolVar(&opts.Verbose, "v", false, "Verbose logging")
	flag.BoolVar(&opts.Debug, "d", false, "Debug logging")

	flag.Parse()
	if opts.Debug {
		opts.Verbose = opts.Debug
	}

	sourceAddr, err := net.ResolveUDPAddr("udp", opts.Source)
	if err != nil {
		logger.Fatalf("Could not resolve source address: %s: %s", opts.Source, err)
	}

	sourceConn, err := net.ListenUDP("udp", sourceAddr)
	if err != nil {
		logger.Fatalf("Could not listen on address: %s: %s", opts.Source, err)
		return
	}

	defer sourceConn.Close()

	url, err := url.Parse(opts.Target)
	query := url.Query()
	query.Set("precision", "s")
	if opts.Bucket != "" {
		query.Set("bucket", opts.Bucket)
	}
	url.RawQuery = query.Encode()

	logger.Printf(">> Starting tempest_influx, Verbose %v Debug %v Source at %v, Target at %v",
		opts.Verbose,
		opts.Debug,
		opts.Source,
		url.String())

	for {
		b := make([]byte, opts.Buffer)
		n, addr, err := sourceConn.ReadFromUDP(b)

		if err != nil {
			logger.Printf("Could not receive a packet from %s: %s", addr, err)
			continue
		}

		if opts.Debug {
			logger.Printf("\nRECV %v %d bytes: %s", addr, n, string(b[:n]))
		}

		line := tempest(logger, addr, b, n)
		if line == "" {
			continue
		}

		if opts.Verbose {
			logger.Printf("POST %s", line)
		}

		request, err := http.NewRequest("POST", url.String(), strings.NewReader(line))
		if err != nil {
			logger.Printf("NewRequest: %v", err)
			continue
		}
		request.Header.Set("Authorization", "Token " + opts.Token)

		client := &http.Client{}
		resp, err := client.Do(request)
		if err != nil {
			logger.Printf("Posting to %s: %v", opts.Target, err)
			continue
		}
		if resp.StatusCode >= 400 {
			logger.Printf("POST: %s", resp.Status)
		}
		resp.Body.Close()
	}
}