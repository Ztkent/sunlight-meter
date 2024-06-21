package pitooth

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/sirupsen/logrus"
)

var l = logrus.New()

func init() {
	// Setup the logger, so it can be parsed by datadog
	l.Formatter = &logrus.JSONFormatter{}
	l.SetOutput(os.Stdout)
	// Set the log level
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		l.SetLevel(logrus.DebugLevel)
	case "info":
		l.SetLevel(logrus.InfoLevel)
	case "error":
		l.SetLevel(logrus.ErrorLevel)
	default:
		l.SetLevel(logrus.InfoLevel)
	}

	// Suppress excess warning logs from the bluetooth library
	logrus.SetLevel(logrus.ErrorLevel)
}

type BluetoothManager interface {
	AcceptConnections() (map[string]Device, error)
	GetNearbyDevices() (map[string]Device, error)
	GetConnectedDevices() (map[string]Device, error)
	Close(bool)
}

type bluetoothManager struct {
	adapter *adapter.Adapter1
	agent   *PiToothAgent
}

type Device struct {
	lastSeen  time.Time
	address   string
	name      string
	connected bool
}

// TODO: Support option design pattern:
// ie WithLogger(l *logrus.Logger), WithAgent(agent agent.Agent), etc.
func NewBluetoothManager(deviceAlias string) (BluetoothManager, error) {
	// We should always set a device alias, or it gets tricky.
	if deviceAlias == "" {
		return nil, fmt.Errorf("Bluetooth device alias cannot be empty")
	}

	// Only support Linux, this should be running on a Raspberry Pi
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Unsupported OS: %v", runtime.GOOS)
	} else {
		_, err := os.Stat("/proc/device-tree/model")
		if err != nil {
			return nil, fmt.Errorf("Not a Raspberry Pi, can't enable Bluetooth Discovery: %v", err)
		}
	}

	// Get the bt adapter to manage bluetooth devices
	defaultAdapter, err := adapter.GetDefaultAdapter()
	if err != nil {
		return nil, fmt.Errorf("Failed to get default adapter: %v", err)
	}
	err = defaultAdapter.SetAlias(deviceAlias)
	if err != nil {
		return nil, fmt.Errorf("Failed to set bluetooth alias: %v", err)
	}
	err = defaultAdapter.SetPowered(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to power on bluetooth adapter: %v", err)
	}

	// Connect a custom bt agent to handle pairing requests
	pitoothAgent := &PiToothAgent{
		SimpleAgent: agent.NewSimpleAgent(),
	}
	err = agent.ExposeAgent(defaultAdapter.Client().GetConnection(), pitoothAgent, agent.CapNoInputNoOutput, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to register agent: %v", err)
	}

	return &bluetoothManager{
		adapter: defaultAdapter,
		agent:   pitoothAgent,
	}, nil
}

func (btm *bluetoothManager) AcceptConnections() (map[string]Device, error) {
	l.Debugln("PiTooth: Starting Pairing...")

	// Make the device discoverable
	l.Debugln("PiTooth: Setting Discoverable...")
	err := btm.adapter.SetDiscoverable(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to make device discoverable: %v", err)
	}

	l.Debugln("PiTooth: Setting Pairable...")
	err = btm.adapter.SetPairable(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to make device pairable: %v", err)
	}

	// Start the discovery
	l.Debugln("PiTooth: Starting Discovery...")
	err = btm.adapter.StartDiscovery()
	if err != nil {
		return nil, fmt.Errorf("Failed to start bluetooth discovery: %v", err)
	}

	// Wait for the device to be discovered
	l.Debugln("PiTooth: Waiting for device to be connected...")
	connectedDevices, err := btm.GetConnectedDevices()
	if err != nil {
		return nil, fmt.Errorf("Failed to get nearby devices: %v", err)
	}

	// Make the device undiscoverable
	l.Debugln("PiTooth: Setting Undiscoverable...")
	err = btm.adapter.SetDiscoverable(false)
	if err != nil {
		return nil, fmt.Errorf("Failed to make device undiscoverable: %v", err)
	}

	// Stop the discovery
	l.Debugln("PiTooth: Stopping Discovery...")
	err = btm.adapter.StopDiscovery()
	if err != nil {
		return nil, fmt.Errorf("Failed to stop bluetooth discovery: %v", err)
	}

	l.Debugln("PiTooth: Connected devices: ", connectedDevices)
	return connectedDevices, nil
}

func (btm *bluetoothManager) GetNearbyDevices() (map[string]Device, error) {
	l.Debugln("PiTooth: Starting GetNearbyDevices...")
	nearbyDevices, err := btm.collectNearbyDevices()
	if err != nil {
		return nil, err
	}

	l.Debugln("PiTooth: # of nearby devices: ", len(nearbyDevices))
	for _, device := range nearbyDevices {
		l.Debugln("PiTooth: Nearby device: ", device.name, " : ", device.address, " : ", device.lastSeen, " : ", device.connected)
	}
	return nearbyDevices, nil
}

func (btm *bluetoothManager) GetConnectedDevices() (map[string]Device, error) {
	l.Debugln("PiTooth: Starting GetConnectedDevices...")
	nearbyDevices, err := btm.collectNearbyDevices()
	if err != nil {
		return nil, err
	}

	connectedDevices := make(map[string]Device)
	for _, device := range nearbyDevices {
		if device.connected {
			connectedDevices[device.address] = device
		}
	}
	l.Debugln("PiTooth: # of connected devices: ", len(connectedDevices))
	return nearbyDevices, nil
}

// Get the devices every 1 second, for 15 seconds.
func (btm *bluetoothManager) collectNearbyDevices() (map[string]Device, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	done := time.After(15 * time.Second)

	nearbyDevices := make(map[string]Device)
	for {
		select {
		case <-done:
			return nearbyDevices, nil
		case <-ticker.C:
			devices, err := btm.adapter.GetDevices()
			if err != nil {
				return nil, fmt.Errorf("Failed to get bluetooth devices: %v", err)
			}
			for _, device := range devices {
				l.Debugln("PiTooth: Discovered bluetooth device: ", device.Properties.Alias, " : ", device.Properties.Address)
				nearbyDevices[device.Properties.Address] = Device{
					lastSeen:  time.Now(),
					address:   device.Properties.Address,
					name:      device.Properties.Alias,
					connected: device.Properties.Connected,
				}
			}
		}
	}
}

func (btm *bluetoothManager) Close(turnOff bool) {
	btm.adapter.StopDiscovery()
	btm.adapter.SetDiscoverable(false)
	btm.adapter.SetPairable(false)
	btm.agent.Cancel()
	if turnOff {
		btm.adapter.SetPowered(false)
	}
}

// func (a *Adapter1) RemoveDevice(device dbus.ObjectPath) error {
// 	return a.client.Call("RemoveDevice", 0, device).Store()
// }

// SetPowered set Powered value
// func (a *Adapter1) SetPowered(v bool) error {
// 	return a.SetProperty("Powered", v)
// }
