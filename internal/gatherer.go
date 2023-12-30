package internal

import (
	"fmt"
	"time"

	"github.com/rspier/go-ecobee/ecobee"
	log "github.com/sirupsen/logrus"
)

type Gatherer struct {
	client       *ecobee.Client
	PollInterval time.Duration

	stopChannel      chan bool
	runtimeRevisions map[string]string
	thermostats      map[string]*ecobee.Thermostat
}

func NewGatherer(client *ecobee.Client) *Gatherer {
	return &Gatherer{
		client:           client,
		PollInterval:     5 * time.Minute,
		stopChannel:      make(chan bool),
		runtimeRevisions: map[string]string{},
		thermostats:      map[string]*ecobee.Thermostat{},
	}
}

func (g *Gatherer) Start() {
	log.Info("Starting Ecobee gatherer...")

	go func() {
		for {
			select {
			case <-g.stopChannel:
				return
			default:
				thermostatsToUpdate, err := g.checkForUpdates()
				if err != nil {
					log.Warn("Failed to check for updates: ", err)
				} else {
					for _, thermostatId := range thermostatsToUpdate {
						err = g.updateThermostat(thermostatId)
						if err != nil {
							log.WithField("thermostatId", thermostatId).Warn("Failed to get update: ", err)
						}
					}
				}

				// Sleep for the defined polling interval
				time.Sleep(g.PollInterval)
			}
		}
	}()
}

func (g *Gatherer) Stop() {
	g.stopChannel <- true
}

func (g *Gatherer) checkForUpdates() ([]string, error) {
	log.Debug("Checking for thermostat updates...")
	var thermostatsToUpdate []string

	SummarySelection := ecobee.Selection{
		SelectionType:          "registered",
		SelectionMatch:         "",
		IncludeEquipmentStatus: true,
	}
	summaries, err := g.client.GetThermostatSummary(SummarySelection)
	if err != nil {
		return nil, fmt.Errorf("failed to get thermostat summary: %w", err)
	}

	for thermostatId, summary := range summaries {
		if g.runtimeRevisions[thermostatId] != summary.RuntimeRevision {
			g.runtimeRevisions[thermostatId] = summary.RuntimeRevision
			thermostatsToUpdate = append(thermostatsToUpdate, thermostatId)
			log.Debugf("Thermostat %s runtime revision changed, needs an update", thermostatId)
		}
	}

	return thermostatsToUpdate, nil
}

func (g *Gatherer) updateThermostat(thermostatId string) error {
	log.Debugf("Updating thermostat %s", thermostatId)
	thermostat, err := g.client.GetThermostat(thermostatId)
	if err != nil {
		return fmt.Errorf("failed to update thermostat %s: %w", thermostatId, err)
	}

	g.thermostats[thermostatId] = thermostat
	return nil
}

func (g *Gatherer) GetThermostats() []*ecobee.Thermostat {
	thermostats := make([]*ecobee.Thermostat, 0, len(g.thermostats))

	for _, thermostat := range g.thermostats {
		thermostats = append(thermostats, thermostat)
	}

	return thermostats
}

func (g *Gatherer) GetThermostat(thermostatId string) *ecobee.Thermostat {
	return g.thermostats[thermostatId]
}

func (g *Gatherer) GetTemperatures() map[string]string {
	temperatures := map[string]string{}
	for _, thermostat := range g.thermostats {
		for _, sensor := range thermostat.RemoteSensors {
			for _, capability := range sensor.Capability {
				if capability.Type == "temperature" {
					temperatures[sensor.Name] = capability.Value
				}
			}
		}
	}
	return temperatures
}

func (g *Gatherer) GetHumidities() map[string]string {
	humidities := map[string]string{}
	for _, thermostat := range g.thermostats {
		for _, sensor := range thermostat.RemoteSensors {
			for _, capability := range sensor.Capability {
				if capability.Type == "humidity" {
					humidities[sensor.Name] = capability.Value
				}
			}
		}
	}
	return humidities
}
