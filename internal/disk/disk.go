package disk

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// GetTemperature reads disk temperature using smartctl
func GetTemperature(device string) (float64, error) {
	// Use the same command pattern as Python version
	// smartctl -A /dev/disk | egrep ^190 | awk '{print $10}'
	cmd := exec.Command("sh", "-c", "smartctl -A "+device+" | egrep '^190' | awk '{print $10}'")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative: look for any temperature line
		cmd = exec.Command("smartctl", "-A", device)
		output, err = cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("smartctl failed: %w", err)
		}

		// Parse smartctl output for temperature
		// Example: "190 Airflow_Temperature_Cel 0x0032   059   036   000    Old_age   Always       -       41"
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Airflow_Temperature_Cel") {
				fields := strings.Fields(line)
				if len(fields) >= 10 {
					// Temperature is in field 10 (index 9)
					temp, err := strconv.ParseFloat(fields[9], 64)
					if err == nil {
						return temp, nil
					}
				}
			}
		}
		return 0, fmt.Errorf("no temperature field found in smartctl output")
	}

	// Parse the direct output from egrep | awk
	tempStr := strings.TrimSpace(string(output))
	if tempStr == "" {
		return 0, fmt.Errorf("no temperature data from smartctl")
	}

	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse temperature '%s': %w", tempStr, err)
	}

	return temp, nil
}

// EnableSATAController enables SATA controller GPIO lines if no disks are detected
func EnableSATAController(sataChip, sataLine1, sataLine2 string) {
	// Check if any sd* disks are already present
	cmd := exec.Command("sh", "-c", "lsblk -d | egrep ^sd | awk '{print $1}'")
	output, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		// Disks already present, no need to toggle power
		log.Println("SATA disks detected, skipping SATA controller enable")
		return
	}

	// No disks detected, enable SATA controller
	if sataChip == "" || sataLine1 == "" || sataLine2 == "" {
		log.Println("SATA controller not configured")
		return
	}

	log.Println("No SATA disks detected, enabling SATA controller...")

	// Default to gpiochip0 if not specified
	if sataChip == "" {
		sataChip = "gpiochip0"
	}

	// If chip is just a number, prepend "gpiochip"
	var chipNum int
	if _, err := fmt.Sscanf(sataChip, "%d", &chipNum); err == nil {
		sataChip = "gpiochip" + sataChip
	}

	// Ensure chip path starts with /dev/
	if !strings.HasPrefix(sataChip, "/dev/") {
		sataChip = "/dev/" + sataChip
	}

	// Convert line strings to ints
	line1Num := 0
	if _, err := fmt.Sscanf(sataLine1, "%d", &line1Num); err != nil {
		log.Printf("Invalid SATA_LINE_1: %s", sataLine1)
		return
	}
	line2Num := 0
	if _, err := fmt.Sscanf(sataLine2, "%d", &line2Num); err != nil {
		log.Printf("Invalid SATA_LINE_2: %s", sataLine2)
		return
	}

	// Enable SATA_LINE_1 (output high)
	l1, err := gpiocdev.RequestLine(sataChip, line1Num, gpiocdev.AsOutput(1))
	if err != nil {
		log.Printf("Failed to request SATA_LINE_1 (line %d): %v", line1Num, err)
	} else {
		defer l1.Close()
		log.Printf("SATA_LINE_1 (line %d) set to HIGH", line1Num)
	}

	// Enable SATA_LINE_2 (output high)
	l2, err := gpiocdev.RequestLine(sataChip, line2Num, gpiocdev.AsOutput(1))
	if err != nil {
		log.Printf("Failed to request SATA_LINE_2 (line %d): %v", line2Num, err)
	} else {
		defer l2.Close()
		log.Printf("SATA_LINE_2 (line %d) set to HIGH", line2Num)
	}

	// Give disks time to spin up
	time.Sleep(2 * time.Second)
	log.Println("SATA controller enabled")
}
