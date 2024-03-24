package cmd

/*
Copyright © 2023 Pete Wall <pete@petewall.net>
*/

import (
	"ecobee-exporter/internal"
	"fmt"
	"os"
	"strings"

	"github.com/sabowski/go-ecobee-kube/ecobee"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "ecobee-exporter",
	Short: "Export metrics from Ecobee Thermostats",
	RunE:  startServer,
}

func startServer(cmd *cobra.Command, args []string) error {
	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug mode enabled")
	}
	log.Info("Welcome to Ecobee Exporter!")

	log.Info("Creating Ecobee client...")
	client, err := createRefreshingEcobeeClient(viper.GetString("ecobee.clientid"), viper.GetString("auth.token-secret"))
	if err != nil {
		return err
	}

	log.Info("Creating Ecobee gatherer...")
	gatherer := internal.NewGatherer(client)
	gatherer.Start()

	log.Info("Creating HTTP server...")
	server := &internal.Server{
		Gatherer: gatherer,
		Port:     viper.GetInt("server.port"),
	}
	return server.Start()
}

func createRefreshingEcobeeClient(clientId, tokenFile string) (*ecobee.Client, error) {
	if clientId == "" {
		return nil, fmt.Errorf("ecobee client id was not defined. Please run again with ECOBEE_CLIENTID defined")
	}
	return ecobee.NewClient(clientId, tokenFile), nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	log.SetLevel(log.InfoLevel)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	rootCmd.Flags().IntP("port", "p", 9500, "Port number to listen on (env: PORT)")
	_ = viper.BindPFlag("server.port", rootCmd.Flags().Lookup("port"))

	rootCmd.Flags().String("ecobeeClientId", "", "Ecobee Client ID (ECOBEE_CLIENTID)")
	_ = viper.BindPFlag("ecobee.clientid", rootCmd.Flags().Lookup("ecobeeClientId"))

	rootCmd.Flags().String("auth-token-secret", "ecobee-token", "File that contains the OAuth token (AUTH_TOKEN-SECRET)")
	_ = viper.BindPFlag("auth.token-secret", rootCmd.Flags().Lookup("auth-token-secret"))

	rootCmd.Flags().Bool("debug", false, "Enable debug logging (env: DEBUG)")
	_ = viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	viper.AutomaticEnv()
}
