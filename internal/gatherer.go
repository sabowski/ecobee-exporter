package internal

/*
Copyright © 2023 Pete Wall <pete@petewall.net>
*/

import (
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rspier/go-ecobee/ecobee"
	log "github.com/sirupsen/logrus"
)

type Gatherer struct {
	client       *ecobee.Client
	PollInterval time.Duration

	stopChannel      chan bool
	runtimeRevisions map[string]string
	thermostats      map[string]*ecobee.Thermostat
	metrics          map[string]*SensorMetrics
}

type SensorMetrics struct {
	thermostatMode           *prometheus.GaugeVec
	temperature              prometheus.Gauge
	desiredCool              prometheus.Gauge
	desiredHeat              prometheus.Gauge
	humidity                 prometheus.Gauge
	desiredHumidity          prometheus.Gauge
	desiredDehumidity        prometheus.Gauge
	airQuality               prometheus.Gauge
	carbonDioxide            prometheus.Gauge
	volatileOrganicCompounds prometheus.Gauge
}

func NewGatherer(client *ecobee.Client) *Gatherer {
	return &Gatherer{
		client:           client,
		PollInterval:     5 * time.Minute,
		stopChannel:      make(chan bool),
		runtimeRevisions: map[string]string{},
		thermostats:      map[string]*ecobee.Thermostat{},
		metrics:          map[string]*SensorMetrics{},
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
	g.updateMetrics(thermostat)
	return nil
}

func (g *Gatherer) updateMetrics(thermostat *ecobee.Thermostat) {
	if thermostat.Runtime.DesiredCool > 0 {
		g.setDesiredCoolMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.DesiredCool))
	}

	if thermostat.Runtime.DesiredHeat > 0 {
		g.setDesiredHeatMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.DesiredHeat))
	}

	if thermostat.Runtime.DesiredHumidity > 0 {
		g.setDesiredHumidityMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.DesiredHumidity))
	}

	if thermostat.Runtime.DesiredDehumidity > 0 {
		g.setDesiredDehumidityMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.DesiredDehumidity))
	}

	if thermostat.Runtime.ActualAQScore > 0 {
		g.setAirQualityMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.ActualAQScore))
	}

	if thermostat.Runtime.ActualCO2 > 0 {
		g.setCarbonDioxideMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.ActualCO2))
	}

	if thermostat.Runtime.ActualVOC > 0 {
		g.setVolatileOrganicCompoundsMetric(thermostat.Name, "thermostat", float64(thermostat.Runtime.ActualVOC))
	}

	g.setThermostatModeMetric(thermostat.Name, "thermostat", thermostat.Events)

	for _, sensor := range thermostat.RemoteSensors {
		for _, capability := range sensor.Capability {
			if g.metrics[sensor.Name] == nil {
				g.metrics[sensor.Name] = &SensorMetrics{}
			}
			if capability.Type == "temperature" && capability.Value != "" && capability.Value != "unknown" {
				value, err := strconv.ParseFloat(capability.Value, 64)
				if err != nil {
					log.Warnf("failed to parse temperature value: \"%s\": %s", capability.Value, err.Error())
				} else {
					g.setTemperatureMetric(sensor.Name, sensor.Type, value)
				}
			}
			if capability.Type == "humidity" && capability.Value != "" && capability.Value != "unknown" {
				value, err := strconv.ParseFloat(capability.Value, 64)
				if err != nil {
					log.Warnf("failed to parse humidity value: \"%s\": %s", capability.Value, err.Error())
				} else {
					g.setHumidityMetric(sensor.Name, sensor.Type, value)
				}
			}
		}
	}
}

func (g *Gatherer) setThermostatModeMetric(sensorName, sensorType string, events []ecobee.Event) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].thermostatMode == nil {
		g.metrics[sensorName].thermostatMode = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "thermostat_mode",
			Help:        "The current mode of the thermostat",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		}, []string{"mode", "running"})
	}
	running := false
	mode := "off"

	for _, event := range events {
		if event.Running {
			if !event.IsHeatOff && !event.IsCoolOff {
				mode = "both"
			} else if !event.IsHeatOff {
				mode = "heat"
			} else if event.IsHeatOff {
				mode = "cool"
			}
			running = true
		}
	}
	g.metrics[sensorName].thermostatMode.With(prometheus.Labels{
		"mode":    mode,
		"running": strconv.FormatBool(running),
	}).Set(1)
}

func (g *Gatherer) setAirQualityMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].airQuality == nil {
		g.metrics[sensorName].airQuality = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "air_quality",
			Help:        "The air quality reported by an Ecobee sensor",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].airQuality.Set(value)
}

func (g *Gatherer) setCarbonDioxideMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].carbonDioxide == nil {
		g.metrics[sensorName].carbonDioxide = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "co2",
			Help:        "The amount of carbon dioxide reported by an Ecobee sensor",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].carbonDioxide.Set(value)
}

func (g *Gatherer) setVolatileOrganicCompoundsMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].volatileOrganicCompounds == nil {
		g.metrics[sensorName].volatileOrganicCompounds = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "voc",
			Help:        "The amount of volatile organic compounds reported by an Ecobee sensor",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].volatileOrganicCompounds.Set(value)
}

func (g *Gatherer) setTemperatureMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].temperature == nil {
		g.metrics[sensorName].temperature = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "temperature_f",
			Help:        "The temperature reported by an Ecobee sensor (in Fahrenheit)",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].temperature.Set(value / 10)
}

func (g *Gatherer) setDesiredCoolMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].desiredCool == nil {
		g.metrics[sensorName].desiredCool = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "desired_cool_f",
			Help:        "The desired temperature to cool to (in Fahrenheit)",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].desiredCool.Set(value / 10)
}

func (g *Gatherer) setDesiredHeatMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].desiredHeat == nil {
		g.metrics[sensorName].desiredHeat = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "desired_heat_f",
			Help:        "The desired temperature to heat to (in Fahrenheit)",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].desiredHeat.Set(value / 10)
}

func (g *Gatherer) setHumidityMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].humidity == nil {
		g.metrics[sensorName].humidity = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "humidity",
			Help:        "The humidity reported by an Ecobee sensor",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].humidity.Set(value)
}

func (g *Gatherer) setDesiredHumidityMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].desiredHumidity == nil {
		g.metrics[sensorName].desiredHumidity = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "desired_humidity",
			Help:        "The desired humidity level reported by an Ecobee sensor",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].desiredHumidity.Set(value)
}

func (g *Gatherer) setDesiredDehumidityMetric(sensorName, sensorType string, value float64) {
	if g.metrics[sensorName] == nil {
		g.metrics[sensorName] = &SensorMetrics{}
	}
	if g.metrics[sensorName].desiredDehumidity == nil {
		g.metrics[sensorName].desiredDehumidity = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "ecobee",
			Subsystem:   "sensor",
			Name:        "desired_dehumidity",
			Help:        "The desired dehumidity level reported by an Ecobee sensor",
			ConstLabels: prometheus.Labels{"name": sensorName, "type": sensorType},
		})
	}
	g.metrics[sensorName].desiredDehumidity.Set(value)
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
