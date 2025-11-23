package disk

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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
