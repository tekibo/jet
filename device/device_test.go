package device

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelectTargetDeviceSingleReadyAutoPicks(t *testing.T) {
	devices := []DeviceInfo{
		{Serial: "offline-1", Status: "offline", Type: "Physical", Model: "Old"},
		{Serial: "393d7b52", Status: "device", Type: "Physical", Model: "24069PC21I"},
	}

	got, err := selectTargetDevice(devices, "", false, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Serial != "393d7b52" {
		t.Fatalf("got serial %q, want 393d7b52", got.Serial)
	}
}

func TestSelectTargetDeviceMultipleInteractivePromptsAndPicks(t *testing.T) {
	devices := []DeviceInfo{
		{Serial: "393d7b52", Status: "device", Type: "Physical", Model: "24069PC21I"},
		{Serial: "emulator-5554", Status: "device", Type: "Emulator", Model: "Pixel_8"},
	}

	var out bytes.Buffer
	got, err := selectTargetDevice(devices, "", true, strings.NewReader("2\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Serial != "emulator-5554" {
		t.Fatalf("got serial %q, want emulator-5554", got.Serial)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "[1] 393d7b52") || !strings.Contains(rendered, "[2] emulator-5554") {
		t.Fatalf("expected numbered device list in prompt, got:\n%s", rendered)
	}
}

func TestSelectTargetDeviceMultipleNonInteractiveListsSerialsInError(t *testing.T) {
	devices := []DeviceInfo{
		{Serial: "393d7b52", Status: "device", Type: "Physical", Model: "24069PC21I"},
		{Serial: "emulator-5554", Status: "device", Type: "Emulator", Model: "Pixel_8"},
	}

	_, err := selectTargetDevice(devices, "", false, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when multiple devices exist in non-interactive mode")
	}
	msg := err.Error()
	if !strings.Contains(msg, "393d7b52") || !strings.Contains(msg, "emulator-5554") || !strings.Contains(msg, "--device") {
		t.Fatalf("expected error listing all ready devices and --device hint, got: %s", msg)
	}
}

func TestSelectTargetDeviceExplicitSerial(t *testing.T) {
	devices := []DeviceInfo{
		{Serial: "393d7b52", Status: "device", Type: "Physical", Model: "24069PC21I"},
		{Serial: "emulator-5554", Status: "unauthorized", Type: "Emulator", Model: "Pixel_8"},
	}

	got, err := selectTargetDevice(devices, "393d7b52", true, strings.NewReader(""), &bytes.Buffer{})
	if err != nil || got.Serial != "393d7b52" {
		t.Fatalf("expected 393d7b52, got (%v, %v)", got, err)
	}

	if _, err := selectTargetDevice(devices, "emulator-5554", true, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for unauthorized explicit device")
	}
}
