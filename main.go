package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)
// create a configuration file conf.go with the following content:

// var hostname = "hostname.example.com" // Change to the address you want to check
// var expectedAddress = "192.168.1.1"  // Change to the expected IP address
// var serviceName = "DnsService" // Change to your Windows service name
// var timeout = 5 * time.Second

func main() {
	// Setup file logging - get executable directory for log file location
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	logPath := filepath.Join(filepath.Dir(exePath), "nsrestart.log")

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// If we can't open the log file, we can't do anything
		return
	}
	defer logFile.Close()

	// Set log output to file only (no stdout for windowsgui builds)
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags) // Include timestamp in logs

	log.Printf("Starting DNS check for %s (expecting %s)", hostname, expectedAddress)

	// Perform DNS lookup with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctx, hostname)

	// Check if lookup failed or returned unexpected address
	lookupFailed := false
	if err != nil || len(addrs) == 0 {
		log.Printf("DNS lookup failed for %s: %v", hostname, err)
		lookupFailed = true
	} else {
		// Check if any of the returned addresses match the expected address
		addressMatched := slices.Contains(addrs, expectedAddress)

		if !addressMatched {
			log.Printf("DNS lookup returned unexpected address(es) for %s: %v (expected: %s)", hostname, addrs, expectedAddress)
			lookupFailed = true
		} else {
			log.Printf("DNS lookup successful for %s: %v (matches expected address)", hostname, addrs)
		}
	}

	if lookupFailed {
		log.Println("Attempting to restart service:", serviceName)

		// Restart Windows service
		cmd := exec.Command("net", "stop", serviceName)
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to stop service: %v", err)
		} else {
			log.Printf("Service %s stopped successfully", serviceName)
		}

		time.Sleep(2 * time.Second)

		cmd = exec.Command("net", "start", serviceName)
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to start service: %v", err)
		} else {
			log.Println("Service restarted successfully")

			// Wait 30 seconds and verify DNS is working
			log.Println("Waiting 15 seconds before verifying DNS resolution...")
			time.Sleep(15 * time.Second)

			// Perform verification DNS lookup
			verifyCtx, verifyCancel := context.WithTimeout(context.Background(), timeout)
			defer verifyCancel()

			verifyAddrs, verifyErr := resolver.LookupHost(verifyCtx, hostname)

			if verifyErr != nil || len(verifyAddrs) == 0 {
				log.Printf("Verification FAILED: DNS lookup still failing for %s: %v", hostname, verifyErr)
			} else if !slices.Contains(verifyAddrs, expectedAddress) {
				log.Printf("Verification FAILED: DNS lookup still returning unexpected address(es) for %s: %v (expected: %s)", hostname, verifyAddrs, expectedAddress)
			} else {
				log.Printf("Verification SUCCESSFUL: DNS lookup now working correctly for %s: %v", hostname, verifyAddrs)
			}
		}
	}
}
