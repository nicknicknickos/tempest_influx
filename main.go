package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var opts *Config

func packet(url *url.URL, addr *net.UDPAddr, b []byte, n int) {
	m, err := tempest(addr, b, n)
	if err != nil {
		log.Printf("%v:", err)
		return
	}
	if m.Timestamp == 0 {
		return
	}
	if opts.Debug {
		log.Printf("InfluxData %+v", m)
	}

	line := m.Marshal()
	if opts.Verbose {
		log.Printf("POST %s", line)
	}

	if m.Bucket != "" {
		// Set query artuments
		query := url.Query()
		query.Set("bucket", m.Bucket)
		url.RawQuery = query.Encode()
	}

	request, err := http.NewRequest("POST", url.String(), strings.NewReader(line))
	if err != nil {
		log.Printf("NewRequest: %v", err)
		return
	}

	if opts.Noop {
		log.Printf("NOOP %s", url)
		return
	}
	if opts.Verbose {
		log.Printf("POST %s", url)
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Printf("Posting to %s: %v", opts.Influx_URL, err)
		return
	}
	if resp.StatusCode >= 400 {
		log.Printf("POST: %s", resp.Status)
	}
	resp.Body.Close()
}

func main() {
	log.SetPrefix("tempest_influx: ")

	// Check for a config path overrride
	var config_dir string = os.Getenv("TEMPEST_INFLUX_CONFIG_DIR")
	if config_dir == "" {
		config_dir = "/config"
	}
	log.Printf("Config Dir: %s", config_dir)
	opts = LoadConfig(config_dir, "tempest_influx")
	if opts.Debug {
		log.Printf("Config %+v", opts)
	}

	sourceAddr, err := net.ResolveUDPAddr("udp", opts.Listen_Address)
	if err != nil {
		log.Fatalf("Could not resolve source address: %s: %s", opts.Listen_Address, err)
	}

	sourceConn, err := net.ListenUDP("udp", sourceAddr)
	if err != nil {
		log.Fatalf("Could not listen on address: %s: %s", opts.Listen_Address, err)
		return
	}

	defer sourceConn.Close()

	url, err := url.Parse(opts.Influx_URL)

	// Set query artuments
	query := url.Query()
	query.Set("precision", "s")
	url.RawQuery = query.Encode()

	log.Printf("Starting tempest_influx, Verbose %v Debug %v Listen_Address %v, Influx %v Bucket %s Rapid_Wind %v Rapid_Wind_Bucket %v",
		opts.Verbose,
		opts.Debug,
		opts.Listen_Address,
		url.String(),
		opts.Influx_Bucket,
		opts.Rapid_Wind,
		opts.Influx_Bucket_Rapid_Wind)

	// Read packets
	for {
		b := make([]byte, opts.Buffer)
		n, addr, err := sourceConn.ReadFromUDP(b)
		if err != nil {
			log.Printf("Could not receive a packet from %s: %s", addr, err)
			continue
		}

		if opts.Debug {
			log.Printf("\nRECV %v %d bytes: %s", addr, n, string(b[:n]))
		}

		go packet(url, addr, b, n)
	}
}
