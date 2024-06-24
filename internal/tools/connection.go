package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pitooth "github.com/Ztkent/pitooth"
)

/*
	 Its essential to have a way to connect a pi to the internet, without the user having to login.
		1. Identify if the pi is connected to the internet.
		2. If its not, turn on the bluetooth adapter.
		3. Allow connections to our pi via bluetooth.
		4. Turn on the obexd service.
		5. Connect to the pi from the client (Via the app)
		6. Send the wifi credentials to the pi, via OPP (Object Push Profile)
		7. Attempt to connect to the wifi network using provided credentials.
*/
const (
	TRANSFER_DIRECTORY = "/home/sunlight/sunlight-meter/transfers"
)

func ManageInternetConnection() {
	if !checkInternetConnection("") {
		log.Println("No internet connection detected. Starting WIFI management...")
		// Create a new Bluetooth manager
		btm, err := pitooth.NewBluetoothManager("SunlightMeter")
		if err != nil {
			log.Println("Failed to create Bluetooth manager:", err)
			return
		}
		defer btm.Close(true)

		// Get the client to connect to the pi w/ bluetooth
		log.Printf("Attempting to accept Bluetooth connections\n")
		for attempt := 1; attempt <= 5; attempt++ {
			connectedDevices, err := btm.AcceptConnections(30 * time.Second)
			if err != nil {
				log.Printf("Attempt %d: Failed to accept Bluetooth connections: %v\n", attempt, err)
			} else if len(connectedDevices) == 0 {
				log.Printf("Attempt %d: No devices connected via Bluetooth\n", attempt)
			} else {
				break
			}
		}

		// Start the OBEX server, accept file transfers
		log.Println("Starting OBEX server")
		if err := btm.ControlOBEXServer(true, TRANSFER_DIRECTORY); err != nil {
			log.Println("Failed to start OBEX server:", err)
			return
		}
		defer btm.ControlOBEXServer(false, "")

		// Watch /transfers for new files
		creds, err := watchForCreds(time.Second * 180)
		if err != nil {
			log.Println("Failed to receive wifi credentials:", err)
			return
		} else if len(creds) == 0 {
			log.Println("No wifi credentials received")
			return
		}

		for _, creds := range creds {
			// Connect to the Wi-Fi network
			log.Println("Attempting to add Wi-Fi network to wpa_supplicant.conf: ", creds.SSID, creds.Password)
			if err := addWifiNetwork(creds.SSID, creds.Password); err != nil {
				log.Println("Failed to add Wi-Fi network:", err)
				return
			}
		}
		logWpaSupplicantContents()

		// Restart the networking service
		log.Println("Restarting networking service")
		cmd := exec.Command("systemctl", "restart", "networking")
		if err := cmd.Run(); err != nil {
			log.Println("Failed to restart networking service:", err)
			return
		}

		// Check if the Pi is connected to the internet
		if !checkInternetConnection("http://www.google.com") {
			log.Println("Failed to connect to Wi-Fi network")
			return
		}
		currentSSID, err := getCurrentSSID()
		if err != nil {
			log.Println("Failed to get current SSID:", err)
		} else {
			log.Printf("Successfully connected to Wi-Fi network: %s\n", currentSSID)
		}
	}
	return
}

func watchForCreds(timeout time.Duration) ([]*Credentials, error) {
	log.Println("Watching for new files in ", TRANSFER_DIRECTORY)

	timeoutTimer := time.NewTimer(timeout)
	retryTicker := time.NewTicker(5 * time.Second)
	defer retryTicker.Stop()
	for {
		select {
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("Timed out waiting for credentials")
		case <-retryTicker.C:
			var err error
			creds, err := processDirectory(TRANSFER_DIRECTORY)
			if err != nil {
				log.Println("Error processing directory:", err)
				return nil, err
			}
			if creds != nil {
				// Credentials found, stop watching
				return creds, nil
			}
		}
	}
}

func cleanUpTransfers() {
	log.Println("Cleaning up transfers directory of .creds files")
	files, _ := filepath.Glob(filepath.Join(TRANSFER_DIRECTORY, "*.creds"))
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			log.Println("Failed to delete .creds file:", file, err)
		}
	}
	return
}

type Credentials struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

func processDirectory(dirPath string) ([]*Credentials, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	// Setup for transfers by cleaning the transfers directory of any existing .creds files
	cleanUpTransfers()

	foundCreds := []*Credentials{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) == ".creds" {
			fullPath := filepath.Join(dirPath, file.Name())
			creds, err := readCredentials(fullPath)
			if err != nil {
				log.Printf("Error reading JSON from %s: %v\n", fullPath, err)
				continue
			}
			if creds.SSID != "" && creds.Password != "" {
				log.Printf("Found credentials in %s: %+v\n", fullPath, creds)
				foundCreds = append(foundCreds, creds)
			}
		}
	}
	return foundCreds, nil
}

func readCredentials(filePath string) (*Credentials, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	err = json.Unmarshal(data, &creds)
	if err != nil {
		return nil, err
	}

	return &creds, nil
}

func checkInternetConnection(testSite string) bool {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	if testSite == "" {
		testSite = "http://www.ztkent.com"
	}
	response, err := client.Get(testSite)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	connected := response.StatusCode == 200
	if connected {
		ssid, err := getCurrentSSID()
		if err != nil {
			log.Println("Failed to get current SSID:", err)
		} else {
			log.Println("Connected to Wi-Fi network:", ssid)
		}
	} else {
		log.Println("Not connected to the internet")
	}

	return connected
}

func getCurrentSSID() (string, error) {
	cmd := exec.Command("iwgetid", "-r")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	ssid := strings.TrimSpace(string(output))
	return ssid, nil
}

// AddWifiNetwork adds a Wi-Fi network configuration to the wpa_supplicant.conf file.
func addWifiNetwork(ssid, password string) error {
	file, err := os.OpenFile("/etc/wpa_supplicant/wpa_supplicant.conf", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	networkConfig := fmt.Sprintf("\nnetwork={\n    ssid=\"%s\"\n    psk=\"%s\"\n    key_mgmt=WPA-PSK\n}\n", ssid, password)
	if _, err = file.WriteString(networkConfig); err != nil {
		return err
	}
	return nil
}

// RemoveWifiNetwork removes a Wi-Fi network configuration from the wpa_supplicant.conf file based on the SSID.
func removeWifiNetwork(ssid string) error {
	cmdStr := fmt.Sprintf("/network={/,/}/ { /ssid=\"%s\"/,/}/d }", ssid)
	cmd := exec.Command("sed", "-i", cmdStr, "/etc/wpa_supplicant/wpa_supplicant.conf")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove Wi-Fi network: %w", err)
	}
	return nil
}

func logWpaSupplicantContents() {
	content, err := os.ReadFile("/etc/wpa_supplicant/wpa_supplicant.conf")
	if err != nil {
		log.Println("Error reading wpa_supplicant.conf:", err)
		return
	}
	log.Println("Contents of wpa_supplicant.conf:")
	log.Println(string(content))
}
