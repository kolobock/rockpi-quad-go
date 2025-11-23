package disk

import (
	"os/exec"
	"strconv"
	"strings"
)

// GetTemperature reads disk temperature using smartctl
func GetTemperature(device string) (float64, error) {
	cmd := exec.Command("smartctl", "-A", device)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Parse smartctl output for temperature
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

	return 0, nil
}
