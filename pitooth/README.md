# PiTooth
Go Bluetooth manager for Raspberry Pi devices.   
Quickly enables simple Bluetooth connectivity and file transfer capabilities.

You can import it into your projects, or use it as a standalone tool.

## Features
- Manage the bluetooth service on the Raspberry Pi.
- Accept incoming Bluetooth connections.
- Discover nearby and connected Bluetooth devices.
- Control the OBEX server to support file transfers.

## Requirements
- Any Raspberry Pi device with Bluetooth
- Go 1.21 or later

To initally setup the Raspberry Pi, you can follow the steps below:
```bash
## Setup Golang
wget https://go.dev/dl/go1.21.11.linux-armv6l.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.11.linux-armv6l.tar.gz && rm go1.21.11.linux-armv6l.tar.gz
echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc

## Add obexd
sudo apt install bluez-obexd
echo 'export PATH=$PATH:/usr/libexec/bluetooth/' >> ~/.bashrc
source ~/.bashrc
```

## Usage

### As a tool
```bash
```

### As a library
```go
    import (
        "log"
        "time"
        "github.com/Ztkent/pitooth"
    )

	// Validate bluetooth capabilities, then create a new Bluetooth Manager
    btm, err := NewBluetoothManager("YourDeviceName")
	if err != nil {
		log.Fatalf("Failed to create Bluetooth Manager: %v", err)
	}

    // Become discoverable, and accept incoming connections for 30 seconds
    connectedDevices, err := btm.AcceptConnections(time.Second * 30)
	if err != nil {
		log.Fatalf("Failed to accept connections: %v", err)
	}

    // Enable the obexd server, and set the file transfer directory
    if err := btm.ControlOBEXServer(true, "/home/sunlight/sunlight-meter"); err != nil {
		log.Fatalf("Failed to start OBEX server: %v", err)
	}

    // At this point, any connected devices can send files to the Raspberry Pi.
```