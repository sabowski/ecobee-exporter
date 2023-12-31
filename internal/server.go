package internal

/*
Copyright © 2023 Pete Wall <pete@petewall.net>
*/

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	Port     int
	Gatherer *Gatherer
}

func (s *Server) Start() error {
	log.Info("Starting HTTP server...")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/humidities", func(w http.ResponseWriter, r *http.Request) {
		payload := s.Gatherer.GetHumidities()
		encoded, _ := json.Marshal(payload)
		_, _ = w.Write(encoded)
	})
	r.Get("/temperatures", func(w http.ResponseWriter, r *http.Request) {
		payload := s.Gatherer.GetTemperatures()
		encoded, _ := json.Marshal(payload)
		_, _ = w.Write(encoded)
	})
	r.Get("/thermostats", func(w http.ResponseWriter, r *http.Request) {
		payload := s.Gatherer.GetThermostats()
		encoded, _ := json.Marshal(payload)
		_, _ = w.Write(encoded)
	})
	r.Get("/thermostats/{thermostatId}", func(w http.ResponseWriter, r *http.Request) {
		thermostatId := chi.URLParam(r, "thermostatId")
		thermostat := s.Gatherer.GetThermostat(thermostatId)
		if thermostat != nil {
			encoded, _ := json.Marshal(thermostat)
			_, _ = w.Write(encoded)
		} else {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, "Thermostat with id \"%s\" not found", thermostatId)
		}
	})

	//http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(fmt.Sprintf(":%d", s.Port), r)
}
