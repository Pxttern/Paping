package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
)

type ConnectionStats struct {
	sync.Mutex
	Attempted int
	Connected int
	Failed    int
	MinTime   time.Duration
	MaxTime   time.Duration
	TotalTime time.Duration
}

type IPInfo struct {
	Org string `json:"org"`
}

var logger = log.New(os.Stdout, "", 0)

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func isValidPort(port int) bool {
	return port >= 0 && port <= 65535
}

func ping(host string, port int, stats *ConnectionStats) {
	stats.Lock()
	defer stats.Unlock()

	startTime := time.Now()

	ipInfo, err := getIPInfo(host)
	if err != nil {
		logger.Printf(color.RedString("Failed to get IP info: %v\n", err))
		stats.Failed++
		return
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Second*5)
	if err != nil {
		logger.Printf(color.RedString("Connection timed out\n"))
		stats.Failed++
		return
	}
	defer conn.Close()

	duration := time.Since(startTime)
	logger.Printf("Connected to "+color.GreenString("%s")+ " time="+color.GreenString("%.2fms")+ " protocol="+color.GreenString("TCP")+ " port="+color.GreenString("%d")+ " ISP="+color.GreenString("%s")+"\n", host, float64(duration.Milliseconds()), port, ipInfo.Org)

	stats.Connected++
	stats.TotalTime += duration

	if stats.MinTime == 0 || duration < stats.MinTime {
		stats.MinTime = duration
	}
	if duration > stats.MaxTime {
		stats.MaxTime = duration
	}
	stats.Attempted++
}

func getIPInfo(ip string) (*IPInfo, error) {
	resp, err := http.Get(fmt.Sprintf("http://ipinfo.io/%s/json", ip))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ipInfo IPInfo
	err = json.NewDecoder(resp.Body).Decode(&ipInfo)
	if err != nil {
		return nil, err
	}

	return &ipInfo, nil
}

func main() {
	if len(os.Args) != 3 {
		logger.Fatal("Usage: go run main.go ip port")
	}

	host := os.Args[1]
	if !isValidIP(host) {
		logger.Fatal("Invalid IP address:", host)
	}

	port, err := strconv.Atoi(os.Args[2])
	if err != nil || !isValidPort(port) {
		logger.Fatal("Invalid port number:", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	stats := &ConnectionStats{}

	go func() {
		<-c
		printReport(stats)
		os.Exit(0)
	}()

	var wg sync.WaitGroup
	for {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ping(host, port, stats)
		}()
		time.Sleep(time.Millisecond * 550)
	}
}

func printReport(stats *ConnectionStats) {
	stats.Lock()
	defer stats.Unlock()

	successRate := float64(stats.Connected) / float64(stats.Attempted) * 100
	logger.Printf("\nConnection statistics:\n")
	logger.Printf("Attempted = "+color.CyanString("%d")+", Connected = "+color.CyanString("%d")+", Failed = "+color.CyanString("%d")+" ("+color.CyanString("%.2f%%")+")\n", stats.Attempted, stats.Connected, stats.Failed, successRate)
	logger.Printf("Approximate connection times:\n")

	if stats.Connected > 0 {
		averageTime := float64(stats.TotalTime.Milliseconds()) / float64(stats.Connected)
		logger.Printf(" Minimum = "+color.CyanString("%.2fms")+", Maximum = "+color.CyanString("%.2fms")+", Average = "+color.CyanString("%.2fms")+"\n", float64(stats.MinTime.Milliseconds()), float64(stats.MaxTime.Milliseconds()), averageTime)
	}
}
