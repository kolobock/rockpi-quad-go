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
	cmd := exec.Command("sh", "-c", "smartctl -A "+device+" | egrep '^190' | awk '{print $10}'")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("smartctl", "-A", device)
		output, err = cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("smartctl failed: %w", err)
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Airflow_Temperature_Cel") {
				fields := strings.Fields(line)
				if len(fields) >= 10 {
					temp, err := strconv.ParseFloat(fields[9], 64)
					if err == nil {
						return temp, nil
					}
				}
			}
		}
		return 0, fmt.Errorf("no temperature field found in smartctl output")
	}

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
	cmd := exec.Command("sh", "-c", "lsblk -d | egrep ^sd | awk '{print $1}'")
	output, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		log.Println("SATA disks detected, skipping SATA controller enable")
		return
	}

	if sataChip == "" || sataLine1 == "" || sataLine2 == "" {
		log.Println("SATA controller not configured")
		return
	}

	log.Println("No SATA disks detected, enabling SATA controller...")

	if sataChip == "" {
		sataChip = "gpiochip0"
	}

	var chipNum int
	if _, err := fmt.Sscanf(sataChip, "%d", &chipNum); err == nil {
		sataChip = "gpiochip" + sataChip
	}

	if !strings.HasPrefix(sataChip, "/dev/") {
		sataChip = "/dev/" + sataChip
	}

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

	l1, err := gpiocdev.RequestLine(sataChip, line1Num, gpiocdev.AsOutput(1))
	if err != nil {
		log.Printf("Failed to request SATA_LINE_1 (line %d): %v", line1Num, err)
	} else {
		defer l1.Close()
		log.Printf("SATA_LINE_1 (line %d) set to HIGH", line1Num)
	}

	l2, err := gpiocdev.RequestLine(sataChip, line2Num, gpiocdev.AsOutput(1))
	if err != nil {
		log.Printf("Failed to request SATA_LINE_2 (line %d): %v", line2Num, err)
	} else {
		defer l2.Close()
		log.Printf("SATA_LINE_2 (line %d) set to HIGH", line2Num)
	}

	time.Sleep(2 * time.Second)
	log.Println("SATA controller enabled")
}
