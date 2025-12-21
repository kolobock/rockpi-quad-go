package oled

import (
	"strings"
	"testing"

	"github.com/kolobock/rockpi-quad-go/internal/config"
)

func TestStripDeviceName(t *testing.T) {
	tests := []struct {
		name   string
		device string
		want   string
	}{
		{"simple device", "/dev/sda1", "sda"},
		{"nvme device", "/dev/nvme0n1p1", "nvme0n1p"},
		{"no partition", "/dev/sdb", "sdb"},
		{"no prefix", "sda1", "sda1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripDeviceName(tt.device)
			if got != tt.want {
				t.Errorf("stripDeviceName(%v) = %v, want %v", tt.device, got, tt.want)
			}
		})
	}
}

func TestTextItem(t *testing.T) {
	item := TextItem{
		X:        10,
		Y:        20,
		Text:     "Test",
		FontSize: 12,
	}

	if item.X != 10 {
		t.Errorf("X = %v, want 10", item.X)
	}
	if item.Text != "Test" {
		t.Errorf("Text = %v, want Test", item.Text)
	}
}

func TestNetworkIOPage(t *testing.T) {
	ctrl := &Controller{
		cfg:      &config.Config{},
		netStats: make(map[string]netIOStats),
	}

	page := &NetworkIOPage{
		ctrl:  ctrl,
		iface: "eth0",
	}
	items := page.GetPageText()

	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}

	if !strings.Contains(items[0].Text, "Network") {
		t.Errorf("first item should contain 'Network', got %v", items[0].Text)
	}
}
