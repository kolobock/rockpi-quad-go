package disk

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/logger"
	"github.com/warthog618/go-gpiocdev"
)

var (
	diskListCache   []string
	lastCheckTime   time.Time
	checkMutex      sync.Mutex
	recheckInterval = 30 * time.Second
	diskTempCache   = make(map[string]float64)
)

// GetSATADisks returns a list of SATA disk devices (/dev/sdX)
func GetSATADisks() []string {
	if len(diskListCache) > 0 {
		return diskListCache
	}

	checkMutex.Lock()
	defer checkMutex.Unlock()

	if lastCheckTime.IsZero() || time.Since(lastCheckTime) > recheckInterval {
		diskListCache = fetchDiskList()
		lastCheckTime = time.Now()
	}

	return diskListCache
}

func fetchDiskList() []string {
	var disks []string
	cmd := exec.Command("sh", "-c", "lsblk -d | egrep ^sd | awk '{print \"/dev/\"$1}'")
	output, err := cmd.Output()
	if err == nil {
		diskList := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, d := range diskList {
			if d != "" {
				disks = append(disks, d)
			}
		}
	}
	return disks
}

func RefreshLastCheckTime() {
	checkMutex.Lock()
	defer checkMutex.Unlock()
	lastCheckTime = time.Time{}
}

// GetTemperature reads disk temperature using smartctl
func GetTemperature(device string) (float64, error) {
	if time.Since(lastCheckTime) < recheckInterval {
		checkMutex.Lock()
		temp, ok := diskTempCache[device]
		checkMutex.Unlock()
		if ok {
			return temp, nil
		}
	}

	checkMutex.Lock()
	defer checkMutex.Unlock()

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

	diskTempCache[device] = temp
	return temp, nil
}

// EnableSATAController enables SATA controller GPIO lines if no disks are detected
func EnableSATAController(sataChip, sataLine1, sataLine2 string) {
	disks := GetSATADisks()
	if len(disks) > 0 {
		logger.Infoln("SATA disks detected, skipping SATA controller enable")
		return
	}

	if sataChip == "" || sataLine1 == "" || sataLine2 == "" {
		logger.Infoln("SATA controller not configured")
		return
	}

	logger.Infoln("No SATA disks detected, enabling SATA controller...")

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
		logger.Errorf("Invalid SATA_LINE_1: %s", sataLine1)
		return
	}
	line2Num := 0
	if _, err := fmt.Sscanf(sataLine2, "%d", &line2Num); err != nil {
		logger.Errorf("Invalid SATA_LINE_2: %s", sataLine2)
		return
	}

	l1, err := gpiocdev.RequestLine(sataChip, line1Num, gpiocdev.AsOutput(1))
	if err != nil {
		logger.Errorf("Failed to request SATA_LINE_1 (line %d): %v", line1Num, err)
	} else {
		defer l1.Close()
		logger.Infof("SATA_LINE_1 (line %d) set to HIGH", line1Num)
	}

	l2, err := gpiocdev.RequestLine(sataChip, line2Num, gpiocdev.AsOutput(1))
	if err != nil {
		logger.Errorf("Failed to request SATA_LINE_2 (line %d): %v", line2Num, err)
	} else {
		defer l2.Close()
		logger.Infof("SATA_LINE_2 (line %d) set to HIGH", line2Num)
	}

	time.Sleep(2 * time.Second)
	logger.Infoln("SATA controller enabled")
}
