package oled

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log/syslog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/internal/disk"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	displayWidth  = 128
	displayHeight = 32
	sliderTime    = 10 * time.Second // default page rotation time
)

// Page represents a displayable page
type Page interface {
	GetPageText() []TextItem
}

// TextItem represents a text element to be drawn
type TextItem struct {
	X    int
	Y    int
	Text string
}

type Controller struct {
	cfg         *config.Config
	dev         *SSD1306
	img         *image.Gray
	mu          sync.Mutex
	pageIndex   int
	pages       []Page
	lastIOTime  time.Time
	lastNetTime time.Time
	netStats    map[string]netIOStats
	diskStats   map[string]diskIOStats
	syslogger   *syslog.Writer
	font        font.Face
}

type netIOStats struct {
	rxBytes   uint64
	txBytes   uint64
	timestamp time.Time
}

type diskIOStats struct {
	readBytes  uint64
	writeBytes uint64
	timestamp  time.Time
}

func New(cfg *config.Config) (*Controller, error) {
	// Create SSD1306 display driver
	display, err := NewSSD1306(displayWidth, displayHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSD1306 display: %w", err)
	}

	// Use basicfont for consistent small display rendering
	c := &Controller{
		cfg:       cfg,
		dev:       display,
		img:       image.NewGray(image.Rect(0, 0, displayWidth, displayHeight)),
		netStats:  make(map[string]netIOStats),
		diskStats: make(map[string]diskIOStats),
		font:      basicfont.Face7x13,
	}

	// Initialize syslog
	logger, err := syslog.New(syslog.LOG_INFO, "rockpi-quad-go")
	if err == nil {
		c.syslogger = logger
	}

	// Initialize network and disk stats
	c.updateNetworkStats()
	c.updateDiskStats()

	// Show welcome message
	c.showWelcome()

	return c, nil
}

func (c *Controller) Run(ctx context.Context, buttonChan <-chan struct{}) error {
	c.showWelcome()

	// Generate all pages
	c.pages = c.generatePages()
	if len(c.pages) == 0 {
		if c.syslogger != nil {
			c.syslogger.Info("No OLED pages configured, display disabled")
		}
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(sliderTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.showGoodbye()
			return nil
		case <-ticker.C:
			c.nextPage()
		case <-buttonChan:
			// Button pressed - advance to next page
			c.nextPage()
		}
	}
}

func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear display
	c.clearImage()
	c.displayToDevice()

	if c.syslogger != nil {
		c.syslogger.Close()
	}
	return c.dev.Close()
}

func (c *Controller) clearImage() {
	for y := 0; y < displayHeight; y++ {
		for x := 0; x < displayWidth; x++ {
			c.img.SetGray(x, y, color.Gray{Y: 0})
		}
	}
}

func (c *Controller) drawText(x, y int, text string) {
	// Python PIL draws from top-left corner at (x, y)
	// Go font.Drawer uses baseline, so we need to add the ascent
	// basicfont.Face7x13 has Ascent of 11 pixels from baseline
	point := fixed.Point26_6{
		X: fixed.I(x),
		Y: fixed.I(y) + fixed.I(11), // Add ascent to convert top to baseline
	}

	d := &font.Drawer{
		Dst:  c.img,
		Src:  image.NewUniform(color.White),
		Face: c.font,
		Dot:  point,
	}
	d.DrawString(text)
}

func (c *Controller) display() error {
	if c.cfg.OLED.Rotate {
		rotated := c.rotateImage180(c.img)
		return c.dev.Display(rotated)
	}
	return c.displayToDevice()
}

func (c *Controller) displayToDevice() error {
	return c.dev.Display(c.img)
}

func (c *Controller) rotateImage180(src *image.Gray) *image.Gray {
	bounds := src.Bounds()
	dst := image.NewGray(bounds)
	w, h := bounds.Dx(), bounds.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, src.At(x, y))
		}
	}
	return dst
}

func (c *Controller) showWelcome() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clearImage()
	c.drawText(0, -2, "ROCKPi QUAD HAT")
	c.drawText(32, 16, "Loading...")
	c.display()
	time.Sleep(1 * time.Second)
}

func (c *Controller) showGoodbye() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clearImage()
	c.drawText(32, 8, "Good Bye ~")
	c.display()
	time.Sleep(2 * time.Second)
	c.clearImage()
	c.display()
}

func (c *Controller) nextPage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pages) == 0 {
		return
	}

	c.pageIndex = (c.pageIndex + 1) % len(c.pages)
	page := c.pages[c.pageIndex]

	c.clearImage()
	items := page.GetPageText()
	for _, item := range items {
		c.drawText(item.X, item.Y, item.Text)
	}
	c.display()
}

func (c *Controller) generatePages() []Page {
	var pages []Page

	// System info pages
	pages = append(pages, &SystemInfoPage0{ctrl: c})
	pages = append(pages, &SystemInfoPage1{ctrl: c})

	// Disk usage page
	if len(c.cfg.Disk.SpaceUsageMountPoints) > 0 {
		pages = append(pages, &DiskUsagePage{ctrl: c})
	}

	// Network I/O pages
	interfaces := c.getNetworkInterfaces()
	for _, iface := range interfaces {
		pages = append(pages, &NetworkIOPage{ctrl: c, iface: iface})
	}

	// Disk I/O pages
	for _, mnt := range c.cfg.Disk.IOUsageMountPoints {
		diskName := c.getDiskNameFromMount(mnt)
		if diskName != "" {
			pages = append(pages, &DiskIOPage{ctrl: c, disk: diskName})
		}
	}

	// Disk temperature page
	if len(c.cfg.Disk.TempDisks) > 0 {
		pages = append(pages, &DiskTempPage{ctrl: c})
	}

	return pages
}

// SystemInfoPage0 - Uptime, CPU Temp, IP Address
type SystemInfoPage0 struct {
	ctrl *Controller
}

func (p *SystemInfoPage0) GetPageText() []TextItem {
	return []TextItem{
		{X: 0, Y: -2, Text: p.ctrl.getUptime()},
		{X: 0, Y: 10, Text: p.ctrl.getCPUTemp()},
		{X: 0, Y: 21, Text: p.ctrl.getIPAddress()},
	}
}

// SystemInfoPage1 - Fan speed, CPU load, Memory usage
type SystemInfoPage1 struct {
	ctrl *Controller
}

func (p *SystemInfoPage1) GetPageText() []TextItem {
	return []TextItem{
		{X: 0, Y: -2, Text: "Fan: monitoring"},
		{X: 0, Y: 10, Text: p.ctrl.getCPULoad()},
		{X: 0, Y: 21, Text: p.ctrl.getMemoryUsage()},
	}
}

// DiskUsagePage - Disk space usage
type DiskUsagePage struct {
	ctrl *Controller
}

func (p *DiskUsagePage) GetPageText() []TextItem {
	items := []TextItem{}
	usage := p.ctrl.getDiskUsage()

	y := -2
	for i, u := range usage {
		if i >= 3 {
			break
		}
		items = append(items, TextItem{X: 0, Y: y, Text: u})
		if i == 0 {
			y = 10
		} else {
			y = 21
		}
	}

	return items
}

// NetworkIOPage - Network I/O rates
type NetworkIOPage struct {
	ctrl  *Controller
	iface string
}

func (p *NetworkIOPage) GetPageText() []TextItem {
	rx, tx := p.ctrl.getNetworkRate(p.iface)
	return []TextItem{
		{X: 0, Y: -2, Text: fmt.Sprintf("Network (%s):", p.iface)},
		{X: 0, Y: 10, Text: fmt.Sprintf("Rx:%10.6f MB/s", rx)},
		{X: 0, Y: 21, Text: fmt.Sprintf("Tx:%10.6f MB/s", tx)},
	}
}

// DiskIOPage - Disk I/O rates
type DiskIOPage struct {
	ctrl *Controller
	disk string
}

func (p *DiskIOPage) GetPageText() []TextItem {
	read, write := p.ctrl.getDiskRate(p.disk)
	return []TextItem{
		{X: 0, Y: -2, Text: fmt.Sprintf("Disk (%s):", p.disk)},
		{X: 0, Y: 10, Text: fmt.Sprintf("R:%11.6f MB/s", read)},
		{X: 0, Y: 21, Text: fmt.Sprintf("W:%11.6f MB/s", write)},
	}
}

// DiskTempPage - Disk temperatures
type DiskTempPage struct {
	ctrl *Controller
}

func (p *DiskTempPage) GetPageText() []TextItem {
	temps := p.ctrl.getDiskTemperatures()
	items := []TextItem{{X: 0, Y: -2, Text: "Disk Temps:"}}

	y := 10
	for i, temp := range temps {
		items = append(items, TextItem{X: 0, Y: y, Text: temp})
		if i == 0 {
			y = 21
		} else {
			break // Only 2 lines fit
		}
	}

	return items
}

// Utility functions to get system information

func (c *Controller) getUptime() string {
	out, err := exec.Command("sh", "-c", "uptime | sed 's/.*up \\([^,]*\\),.*/\\1/'").Output()
	if err != nil {
		return "Uptime: N/A"
	}
	return "Up: " + strings.TrimSpace(string(out))
}

func (c *Controller) getCPUTemp() string {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return "CPU: N/A"
	}
	temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return "CPU: N/A"
	}
	temp = temp / 1000.0

	if c.cfg.OLED.Fahrenheit {
		return fmt.Sprintf("CPU: %.0f°F", temp*1.8+32)
	}
	return fmt.Sprintf("CPU: %.1f°C", temp)
}

func (c *Controller) getIPAddress() string {
	out, err := exec.Command("hostname", "-I").Output()
	if err != nil {
		return "IP: N/A"
	}
	fields := strings.Fields(string(out))
	if len(fields) > 0 {
		return "IP: " + fields[0]
	}
	return "IP: N/A"
}

func (c *Controller) getCPULoad() string {
	out, err := exec.Command("sh", "-c", "uptime | awk '{print $(NF-2)}'").Output()
	if err != nil {
		return "CPU Load: N/A"
	}
	load := strings.TrimSpace(string(out))
	load = strings.TrimSuffix(load, ",")
	return "CPU: " + load
}

func (c *Controller) getMemoryUsage() string {
	out, err := exec.Command("sh", "-c", "free -m | awk 'NR==2{printf \"%s/%sMB\", $3,$2}'").Output()
	if err != nil {
		return "Mem: N/A"
	}
	return "Mem: " + strings.TrimSpace(string(out))
}

func (c *Controller) getDiskUsage() []string {
	var usage []string

	// Add root partition
	out, err := exec.Command("sh", "-c", "df -h / | awk 'NR==2{printf \"%s\", $5}'").Output()
	if err == nil {
		usage = append(usage, "/ "+strings.TrimSpace(string(out)))
	}

	// Add configured mount points
	for _, mnt := range c.cfg.Disk.SpaceUsageMountPoints {
		cmd := fmt.Sprintf("df -h %s | awk 'NR==2{printf \"%%s\", $5}'", mnt)
		out, err := exec.Command("sh", "-c", cmd).Output()
		if err == nil && len(out) > 0 {
			usage = append(usage, mnt+" "+strings.TrimSpace(string(out)))
		}
	}

	return usage
}

func (c *Controller) getNetworkInterfaces() []string {
	// Check if specific interfaces are configured
	if len(c.cfg.Disk.IOUsageMountPoints) > 0 {
		// Use interfaces from config if available
		// For now, return common interfaces
	}

	// Default to eth0 and wlan0 if they exist
	var interfaces []string
	for _, iface := range []string{"eth0", "wlan0", "enp0s3"} {
		if _, err := os.Stat("/sys/class/net/" + iface); err == nil {
			interfaces = append(interfaces, iface)
		}
	}
	return interfaces
}

func (c *Controller) updateNetworkStats() {
	interfaces := c.getNetworkInterfaces()
	for _, iface := range interfaces {
		path := "/sys/class/net/" + iface + "/statistics/"

		rxData, _ := os.ReadFile(path + "rx_bytes")
		txData, _ := os.ReadFile(path + "tx_bytes")

		rx, _ := strconv.ParseUint(strings.TrimSpace(string(rxData)), 10, 64)
		tx, _ := strconv.ParseUint(strings.TrimSpace(string(txData)), 10, 64)

		c.netStats[iface] = netIOStats{
			rxBytes:   rx,
			txBytes:   tx,
			timestamp: time.Now(),
		}
	}
}

func (c *Controller) getNetworkRate(iface string) (float64, float64) {
	oldStats, exists := c.netStats[iface]
	if !exists {
		c.updateNetworkStats()
		return 0, 0
	}

	path := "/sys/class/net/" + iface + "/statistics/"
	rxData, _ := os.ReadFile(path + "rx_bytes")
	txData, _ := os.ReadFile(path + "tx_bytes")

	rx, _ := strconv.ParseUint(strings.TrimSpace(string(rxData)), 10, 64)
	tx, _ := strconv.ParseUint(strings.TrimSpace(string(txData)), 10, 64)

	now := time.Now()
	elapsed := now.Sub(oldStats.timestamp).Seconds()

	rxRate := float64(rx-oldStats.rxBytes) / elapsed / 1024 / 1024
	txRate := float64(tx-oldStats.txBytes) / elapsed / 1024 / 1024

	c.netStats[iface] = netIOStats{
		rxBytes:   rx,
		txBytes:   tx,
		timestamp: now,
	}

	return rxRate, txRate
}

func (c *Controller) getDiskNameFromMount(mount string) string {
	out, err := exec.Command("sh", "-c", fmt.Sprintf("df %s | awk 'NR==2{print $1}'", mount)).Output()
	if err != nil {
		return ""
	}
	device := strings.TrimSpace(string(out))
	// Extract disk name (e.g., /dev/sda1 -> sda)
	if strings.HasPrefix(device, "/dev/") {
		device = strings.TrimPrefix(device, "/dev/")
		// Remove partition number
		for i := len(device) - 1; i >= 0; i-- {
			if device[i] < '0' || device[i] > '9' {
				return device[:i+1]
			}
		}
	}
	return device
}

func (c *Controller) updateDiskStats() {
	for _, mnt := range c.cfg.Disk.IOUsageMountPoints {
		diskName := c.getDiskNameFromMount(mnt)
		if diskName == "" {
			continue
		}

		path := "/sys/block/" + diskName + "/"
		readData, _ := os.ReadFile(path + "stat")

		if len(readData) > 0 {
			fields := strings.Fields(string(readData))
			if len(fields) >= 10 {
				readSectors, _ := strconv.ParseUint(fields[2], 10, 64)
				writeSectors, _ := strconv.ParseUint(fields[6], 10, 64)

				c.diskStats[diskName] = diskIOStats{
					readBytes:  readSectors * 512,
					writeBytes: writeSectors * 512,
					timestamp:  time.Now(),
				}
			}
		}
	}
}

func (c *Controller) getDiskRate(diskName string) (float64, float64) {
	oldStats, exists := c.diskStats[diskName]
	if !exists {
		c.updateDiskStats()
		return 0, 0
	}

	path := "/sys/block/" + diskName + "/stat"
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 10 {
		return 0, 0
	}

	readSectors, _ := strconv.ParseUint(fields[2], 10, 64)
	writeSectors, _ := strconv.ParseUint(fields[6], 10, 64)

	now := time.Now()
	elapsed := now.Sub(oldStats.timestamp).Seconds()

	readRate := float64(readSectors*512-oldStats.readBytes) / elapsed / 1024 / 1024
	writeRate := float64(writeSectors*512-oldStats.writeBytes) / elapsed / 1024 / 1024

	c.diskStats[diskName] = diskIOStats{
		readBytes:  readSectors * 512,
		writeBytes: writeSectors * 512,
		timestamp:  now,
	}

	return readRate, writeRate
}

func (c *Controller) getDiskTemperatures() []string {
	var temps []string

	for _, diskDev := range c.cfg.Disk.TempDisks {
		temp, err := disk.GetTemperature(diskDev)
		if err == nil && temp > 0 {
			diskName := strings.TrimPrefix(diskDev, "/dev/")
			temps = append(temps, fmt.Sprintf("%s %.0f°C", diskName, temp))
		}
	}

	return temps
}
