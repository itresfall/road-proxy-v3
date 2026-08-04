package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type captureOptions struct {
	AdvancedCapture advancedCaptureMode
}

type advancedCaptureSession interface {
	Finish(summary *captureSummary) (*advancedCaptureReport, []capturedPacket, error)
	Abort()
}

func normalizedCaptureMode(mode advancedCaptureMode) advancedCaptureMode {
	if mode == "" {
		return advancedCaptureAuto
	}
	return mode
}

func startAdvancedCapture(mode advancedCaptureMode) (advancedCaptureSession, *advancedCaptureReport, error) {
	mode = normalizedCaptureMode(mode)
	if mode == advancedCaptureOff {
		return nil, &advancedCaptureReport{
			RequestedMode: string(mode),
			Status:        "disabled",
			Note:          "advanced packet capture was disabled by the user",
		}, nil
	}

	if runtime.GOOS != "windows" {
		report := &advancedCaptureReport{
			RequestedMode: string(mode),
			Status:        "unavailable",
			Note:          "advanced packet capture is not implemented for this platform; socket snapshot fallback was used",
		}
		if mode == advancedCaptureRequired {
			return nil, report, fmt.Errorf("advanced packet capture is unavailable on %s", runtime.GOOS)
		}
		return nil, report, nil
	}

	packetMonitor, report, err := startPktmonCapture(mode)
	if err == nil {
		return packetMonitor, report, nil
	}
	if mode == advancedCaptureRequired {
		return nil, report, err
	}
	return nil, report, nil
}

type pktmonCaptureSession struct {
	binary   string
	dir      string
	etlPath  string
	pcapPath string
	mode     advancedCaptureMode
	stopped  bool
}

func startPktmonCapture(mode advancedCaptureMode) (*pktmonCaptureSession, *advancedCaptureReport, error) {
	report := &advancedCaptureReport{
		RequestedMode: string(mode),
		Backend:       "pktmon",
		Status:        "unavailable",
	}
	binary, err := exec.LookPath("pktmon")
	if err != nil {
		report.Note = "pktmon was not found; socket snapshot fallback was used"
		return nil, report, fmt.Errorf("pktmon not found")
	}

	dir, err := os.MkdirTemp("", "road-plugin-studio-pktmon-")
	if err != nil {
		report.Note = "temporary packet capture storage could not be created; socket snapshot fallback was used"
		return nil, report, fmt.Errorf("create pktmon temp directory: %w", err)
	}
	session := &pktmonCaptureSession{
		binary:   binary,
		dir:      dir,
		etlPath:  filepath.Join(dir, "capture.etl"),
		pcapPath: filepath.Join(dir, "capture.pcapng"),
		mode:     mode,
	}

	// Capture only a small prefix. Pcapng still records original packet length,
	// while this keeps temporary raw packet exposure and file growth bounded.
	if _, err := runPktmonCommand(binary,
		"start",
		"--capture",
		"--pkt-size", "256",
		"--flags", "0x010",
		"--file-name", session.etlPath,
		"--file-size", "32",
		"--log-mode", "circular",
	); err != nil {
		session.cleanup()
		report.Note = "pktmon could not start (an elevated terminal may be required, or pktmon may already be in use); socket snapshot fallback was used"
		return nil, report, err
	}

	report.Status = "running"
	report.Note = "pktmon captured a temporary 256-byte packet prefix; raw capture files are deleted after summary parsing"
	return session, report, nil
}

func (s *pktmonCaptureSession) Finish(summary *captureSummary) (*advancedCaptureReport, []capturedPacket, error) {
	report := &advancedCaptureReport{
		RequestedMode: string(s.mode),
		Backend:       "pktmon",
		Status:        "failed",
	}
	if s == nil {
		report.Note = "pktmon session was not initialized"
		return report, nil, fmt.Errorf("pktmon session was not initialized")
	}
	defer s.cleanup()

	if err := s.stop(); err != nil {
		report.Note = "pktmon could not stop cleanly; socket snapshot results were kept"
		return report, nil, err
	}
	if _, err := runPktmonCommand(s.binary, "etl2pcap", s.etlPath, "--out", s.pcapPath); err != nil {
		report.Note = "pktmon capture could not be converted to pcapng; socket snapshot results were kept"
		return report, nil, err
	}
	packets, err := parsePCAPNGFile(s.pcapPath)
	if err != nil {
		report.Note = "pktmon pcapng output could not be parsed; socket snapshot results were kept"
		return report, nil, err
	}
	matched := filterCapturedPacketsForSummary(packets, summary)
	report.CapturedPackets = len(packets)
	report.MatchedPackets = len(matched)
	if len(matched) == 0 {
		report.Status = "captured_no_matching_packets"
		report.Note = "pktmon completed, but no packets matched ports seen in the selected process snapshots"
		return report, matched, nil
	}
	report.Status = "captured"
	report.Note = "pktmon packet metadata was merged with socket snapshots; raw capture files were deleted"
	return report, matched, nil
}

func (s *pktmonCaptureSession) Abort() {
	if s == nil {
		return
	}
	_ = s.stop()
	s.cleanup()
}

func (s *pktmonCaptureSession) stop() error {
	if s == nil || s.stopped {
		return nil
	}
	s.stopped = true
	_, err := runPktmonCommand(s.binary, "stop")
	return err
}

func (s *pktmonCaptureSession) cleanup() {
	if s == nil || s.dir == "" {
		return
	}
	_ = os.RemoveAll(s.dir)
	s.dir = ""
}

func runPktmonCommand(binary string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if ctx.Err() != nil {
		return output, fmt.Errorf("pktmon command timed out")
	}
	if err != nil {
		message := strings.Join(strings.Fields(string(output)), " ")
		if len(message) > 240 {
			message = message[:240]
		}
		if message == "" {
			message = err.Error()
		}
		return output, fmt.Errorf("pktmon command failed: %s", message)
	}
	return output, nil
}

func printAdvancedCaptureStatus(report *advancedCaptureReport) {
	if report == nil {
		return
	}
	backend := report.Backend
	if backend == "" {
		backend = "socket_snapshot"
	}
	status := sm("studio.advanced_capture_status." + report.Status)
	if status == "studio.advanced_capture_status."+report.Status {
		status = report.Status
	}
	fmt.Printf(sm("studio.advanced_capture_status"), backend, status)
	if strings.TrimSpace(report.Note) != "" {
		fmt.Printf(sm("studio.advanced_capture_note"), report.Note)
	}
}
