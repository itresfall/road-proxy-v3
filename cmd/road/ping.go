package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
)

type pingResponse struct {
	Service        string `json:"service"`
	Plane          string `json:"plane"`
	ServerTime     string `json:"server_time"`
	ServerUnixNano int64  `json:"server_unix_nano"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
}

type pingSample struct {
	Seq      int
	RTT      time.Duration
	Response pingResponse
}

type pingSummary struct {
	Count int
	Min   time.Duration
	Avg   time.Duration
	Max   time.Duration
}

func runPingCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	fs.SetOutput(out)

	rawURL := fs.String("url", "", msg("ping.flag_url"))
	clientPath := fs.String("client", "configs/client.json", msg("ping.flag_client"))
	count := fs.Int("count", 4, msg("ping.flag_count"))
	intervalRaw := fs.String("interval", "1s", msg("ping.flag_interval"))
	timeoutRaw := fs.String("timeout", "3s", msg("ping.flag_timeout"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf(msg("ping.unexpected_args"), strings.Join(fs.Args(), " "))
	}
	if *count < 1 {
		return fmt.Errorf(msg("ping.count_invalid"), *count)
	}
	interval, err := time.ParseDuration(*intervalRaw)
	if err != nil || interval < 0 {
		return fmt.Errorf(msg("ping.interval_invalid"), *intervalRaw)
	}
	timeout, err := time.ParseDuration(*timeoutRaw)
	if err != nil || timeout <= 0 {
		return fmt.Errorf(msg("ping.timeout_invalid"), *timeoutRaw)
	}

	wsURL, err := pingSourceWSURL(*rawURL, *clientPath)
	if err != nil {
		return err
	}
	base, err := apiBaseFromWSURL(wsURL)
	if err != nil {
		return err
	}
	endpoint := base + "/api/ping"
	httpClient := &http.Client{Timeout: timeout}

	samples := make([]pingSample, 0, *count)
	for i := 1; i <= *count; i++ {
		sample, err := measurePing(httpClient, endpoint, i)
		if err != nil {
			fmt.Fprintf(out, msg("ping.sample_error"), i, err)
		} else {
			samples = append(samples, sample)
			fmt.Fprintf(out, msg("ping.sample_ok"), i, sample.RTT.Truncate(time.Microsecond), sample.Response.Plane, sample.Response.ServerTime)
		}
		if i < *count && interval > 0 {
			time.Sleep(interval)
		}
	}
	if len(samples) == 0 {
		return fmt.Errorf(msg("ping.no_success"))
	}

	summary := summarizePing(samples)
	fmt.Fprintf(out, msg("ping.summary"), summary.Count, summary.Min.Truncate(time.Microsecond), summary.Avg.Truncate(time.Microsecond), summary.Max.Truncate(time.Microsecond))
	return nil
}

func pingSourceWSURL(rawURL, clientPath string) (string, error) {
	if strings.TrimSpace(rawURL) != "" {
		return normalizeClientWSURL(rawURL)
	}
	cfg, err := config.LoadClient(app.ResolveExistingPath(clientPath))
	if err != nil {
		return "", err
	}
	return normalizeClientWSURL(cfg.ServerWSURL)
}

func measurePing(httpClient *http.Client, endpoint string, seq int) (pingSample, error) {
	start := time.Now()
	resp, err := httpClient.Get(endpoint)
	rtt := time.Since(start)
	if err != nil {
		return pingSample{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return pingSample{}, fmt.Errorf("http %d", resp.StatusCode)
	}

	var body pingResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return pingSample{}, err
	}
	return pingSample{Seq: seq, RTT: rtt, Response: body}, nil
}

func summarizePing(samples []pingSample) pingSummary {
	if len(samples) == 0 {
		return pingSummary{}
	}
	minRTT := samples[0].RTT
	maxRTT := samples[0].RTT
	var total time.Duration
	for _, sample := range samples {
		if sample.RTT < minRTT {
			minRTT = sample.RTT
		}
		if sample.RTT > maxRTT {
			maxRTT = sample.RTT
		}
		total += sample.RTT
	}
	return pingSummary{
		Count: len(samples),
		Min:   minRTT,
		Avg:   total / time.Duration(len(samples)),
		Max:   maxRTT,
	}
}
