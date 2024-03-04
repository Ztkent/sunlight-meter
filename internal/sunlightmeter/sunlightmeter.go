package sunlightmeter

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Ztkent/sunlight-meter/internal/tools"
	"github.com/Ztkent/sunlight-meter/tsl2591"
	"github.com/google/uuid"
)

type SLMeter struct {
	*tsl2591.TSL2591
	LuxResultsChan chan LuxResults
	ResultsDB      *sql.DB
	cancel         context.CancelFunc
}

type LuxResults struct {
	Lux          float64
	Infrared     float64
	Visible      float64
	FullSpectrum float64
	JobID        string
}

/*
TSL2591_VISIBLE      byte = 2 ///< channel 0 - channel 1
TSL2591_INFRARED     byte = 1 ///< channel 1
TSL2591_FULLSPECTRUM byte = 0 ///< channel 0
*/

const (
	MAX_JOB_DURATION = 3 * time.Minute
)

// Start the sensor, and collect data in a loop
func (m *SLMeter) Start() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.Enabled {
			http.Error(w, "The sensor is already running", http.StatusConflict)
			return
		}

		// Create a new context with a timeout to manage the sensor lifecycle
		ctx, cancel := context.WithTimeout(r.Context(), MAX_JOB_DURATION)
		m.cancel = cancel

		// Enable the sensor
		m.Enable()
		defer m.Disable()

		jobID := uuid.New().String()
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ctx.Done():
				log.Println("Job Cancelled, stopping sensor")
				return
			default:
			}

			// Read the sensor
			ch0, ch1, err := m.GetFullLuminosity()
			if err != nil {
				http.Error(w, "The sensor failed to get luminosity", http.StatusConflict)
				return
			}
			tools.DebugLog(fmt.Sprintf("0x%04x 0x%04x\n", ch0, ch1))

			// Calculate the lux value from the sensor readings
			lux, err := m.CalculateLux(ch0, ch1)
			if err != nil {
				log.Fatal(err)
			}

			// Send the results to the LuxResultsChan
			m.LuxResultsChan <- LuxResults{
				Lux:          lux,
				Visible:      m.GetNormalizedOutput(tsl2591.TSL2591_VISIBLE, ch0, ch1),
				Infrared:     m.GetNormalizedOutput(tsl2591.TSL2591_INFRARED, ch0, ch1),
				FullSpectrum: m.GetNormalizedOutput(tsl2591.TSL2591_FULLSPECTRUM, ch0, ch1),
				JobID:        jobID,
			}

			log.Println(lux)
			<-ticker.C
		}
	}
}

func (m *SLMeter) Stop() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.Enabled {
			http.Error(w, "The sensor is already stopped", http.StatusConflict)
			return
		}
		defer m.Disable()
		m.cancel()
	}
}

// Read from LuxResultsChan, write the results to sqlite
func (m *SLMeter) WriteToDB() {
	for {
		select {
		case result := <-m.LuxResultsChan:
			log.Println("Received result message: ", result)
			_, err := m.ResultsDB.Exec(
				"INSERT INTO sunlight (job_id, lux, full_spectrum, visible, infrared) VALUES (?, ?, ?, ?, ?)",
				result.JobID, result.Lux, result.FullSpectrum, result.Visible, result.Infrared,
			)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
