package pitooth

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	// TODO: Maybe https://github.com/tinygo-org/bluetooth is better...
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
)

func EnableBluetooth() error {
	log.Println("Configuration: Starting Bluetooth...")
	// Only support Linux, this should be running on a Raspberry Pi
	if runtime.GOOS != "linux" {
		return fmt.Errorf("Unsupported OS: %v", runtime.GOOS)
	} else {
		_, err := os.Stat("/proc/device-tree/model")
		if err != nil {
			return fmt.Errorf("Not a Raspberry Pi, can't enable Bluetooth Discovery: %v", err)
		}
	}

	// Get the default adapter
	defaultAdapter, err := adapter.GetDefaultAdapter()
	if err != nil {
		return fmt.Errorf("Failed to get default adapter: %v", err)
	}

	address, err := defaultAdapter.GetAddress()
	if err != nil {
		return fmt.Errorf("Failed to get device address: %v", err)
	}
	log.Printf("Device address: %s", address)

	err = defaultAdapter.SetAlias("SunlightMeter")
	if err != nil {
		return fmt.Errorf("Failed to set name: %v", err)
	}

	// Register an agent to handle pairing requests
	log.Println("Configuration: Registering Agent...")
	// err = defaultAdapter.RegisterAgent()

	// Make the device discoverable
	log.Println("Configuration: Setting Discoverable...")
	err = defaultAdapter.SetDiscoverable(true)
	if err != nil {
		return fmt.Errorf("Failed to make device discoverable: %v", err)
	}

	// Start the discovery
	log.Println("Configuration: Starting Discovery...")
	err = defaultAdapter.StartDiscovery()
	if err != nil {
		return fmt.Errorf("Failed to start bluetooth discovery: %v", err)
	}

	// Map to remember devices we've seen in the last 15 seconds.
	// Sometimes we dont see them all in a single scan
	for {
		// Get discovered devices
		log.Println("Configuration: After Discovery - ")
		devices, err := defaultAdapter.GetDevices()
		if err != nil {
			return fmt.Errorf("Failed to get bluetooth devices: %v", err)
		}

		// Log them
		log.Println("Configuration: After GetDevices - ")
		for _, device := range devices {
			log.Printf("Discovered bluetooth device: %s : %v", device.Properties.Alias, device.Properties.Address)
			log.Printf("Properties: %v", device.Properties)
		}
		time.Sleep(15 * time.Second)
	}
}
