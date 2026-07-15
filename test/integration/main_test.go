package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestIntegrationSmoke(t *testing.T) {
	port := findFreePort(t)

	serverBin := filepath.Join(os.TempDir(), "test-server")
	buildCmd := exec.Command("go", "build", "-o", serverBin, "../../cmd/server")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Build server: %v\n%s", err, out)
	}
	defer os.Remove(serverBin)

	serverCmd := exec.Command(serverBin, "--addr=127.0.0.1:"+port)
	serverCmd.Stdout = nil
	serverCmd.Stderr = nil

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	defer func() {
		if err := serverCmd.Process.Signal(os.Interrupt); err != nil {
			t.Logf("Signal server: %v", err)
		}
		if err := serverCmd.Wait(); err != nil {
			t.Logf("Wait server: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	clientCmd := exec.Command("go", "run", "../../cmd/client",
		"--server=127.0.0.1:"+port,
		"--packets=100",
		"--workers=2",
		"--timeout=5s",
	)

	output, err := clientCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Client error: %v\nOutput:\n%s", err, string(output))
	}

	outputStr := string(output)

	clientRe := regexp.MustCompile(`\[CLIENT\] Seq=(\d+).*?Status=DELIVERED`)
	clientMatches := clientRe.FindAllStringSubmatch(outputStr, -1)

	if len(clientMatches) == 0 {
		t.Fatal("No client DELIVERED messages found")
	}

	seen := make(map[int]bool)
	var seqNumbers []int
	for _, match := range clientMatches {
		seq, _ := strconv.Atoi(match[1])
		if !seen[seq] {
			seen[seq] = true
			seqNumbers = append(seqNumbers, seq)
		}
	}

	for i := 1; i < len(seqNumbers); i++ {
		if seqNumbers[i] <= seqNumbers[i-1] {
			t.Errorf("Ordered output broken: %d after %d", seqNumbers[i], seqNumbers[i-1])
		}
	}

	deliveredCount := len(seqNumbers)
	if deliveredCount != 100 {
		t.Errorf("Expected 100 DELIVERED, got %d", deliveredCount)
	}

	ackedRe := regexp.MustCompile(`Acked:\s+(\d+)`)
	ackedMatches := ackedRe.FindStringSubmatch(outputStr)
	if len(ackedMatches) > 0 {
		acked, _ := strconv.Atoi(ackedMatches[1])
		if acked < 100 {
			t.Errorf("Expected at least 100 acked, got %d", acked)
		}
	} else {
		t.Error("No Acked stat found")
	}

	t.Log("All packets delivered, no losses")
}

func TestLoad10kPackets(t *testing.T) {
	port := findFreePort(t)

	serverBin := filepath.Join(os.TempDir(), "test-server")
	buildCmd := exec.Command("go", "build", "-o", serverBin, "../../cmd/server")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Build server: %v\n%s", err, out)
	}
	defer os.Remove(serverBin)

	serverCmd := exec.Command(serverBin, "--addr=127.0.0.1:"+port, "--workers=8")
	serverCmd.Stdout = nil
	serverCmd.Stderr = nil

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	defer func() {
		if err := serverCmd.Process.Signal(os.Interrupt); err != nil {
			t.Logf("Signal server: %v", err)
		}
		if err := serverCmd.Wait(); err != nil {
			t.Logf("Wait server: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	startTime := time.Now()

	clientCmd := exec.Command("go", "run", "../../cmd/client",
		"--server=127.0.0.1:"+port,
		"--packets=10000",
		"--workers=10",
		"--timeout=5s",
	)

	output, err := clientCmd.CombinedOutput()
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Client error: %v\nOutput:\n%s", err, string(output))
	}

	outputStr := string(output)
	t.Logf("Test completed in %v", elapsed)

	lossRe := regexp.MustCompile(`Loss:\s+([\d.]+)%`)
	lossMatches := lossRe.FindStringSubmatch(outputStr)

	if len(lossMatches) > 0 {
		lossPercent, _ := strconv.ParseFloat(lossMatches[1], 64)
		t.Logf("Loss rate: %.2f%%", lossPercent)

		if lossPercent > 3.0 {
			t.Errorf("Loss rate %.2f%% exceeds 3%% threshold", lossPercent)
		}
	}

	clientRe := regexp.MustCompile(`\[CLIENT\] Seq=(\d+).*?Status=DELIVERED`)
	clientMatches := clientRe.FindAllStringSubmatch(outputStr, -1)

	seen := make(map[int]bool)
	var seqNumbers []int
	for _, match := range clientMatches {
		seq, _ := strconv.Atoi(match[1])
		if !seen[seq] {
			seen[seq] = true
			seqNumbers = append(seqNumbers, seq)
		}
	}

	t.Logf("Delivered packets: %d", len(seqNumbers))

	for i := 1; i < len(seqNumbers); i++ {
		if seqNumbers[i] <= seqNumbers[i-1] {
			t.Errorf("Ordered output broken: %d after %d", seqNumbers[i], seqNumbers[i-1])
		}
	}
}

func findFreePort(t *testing.T) string {
	for port := 9000; port <= 9100; port++ {
		cmd := exec.Command("lsof", "-t", "-i:"+strconv.Itoa(port))
		output, _ := cmd.Output()
		if len(output) == 0 {
			return strconv.Itoa(port)
		}
	}
	t.Fatal("No free port found")
	return ""
}
