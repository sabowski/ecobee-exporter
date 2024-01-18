package cmd

/*
Copyright © 2023 Pete Wall <pete@petewall.net>
*/

import (
	"context"
	"ecobee-exporter/internal"
	"encoding/json"
	"fmt"
	"github.com/rspier/go-ecobee/ecobee"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"os"
	"strings"
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
	var client *ecobee.Client
	var err error
	if viper.GetBool("auth.handle-refresh") {
		client, err = createRefreshingEcobeeClient(viper.GetString("ecobee.clientid"), viper.GetString("auth.token-file"))
	} else {
		client, err = createNonRefreshingEcobeeClient(viper.GetString("auth.token-file"))
	}
	if err != nil {
		return err
	}

	log.Info("Creating Ecobee gatherer...")
	gatherer := internal.NewGatherer(client)
	gatherer.Start()

	log.Info("Creating HTTP server...")
	server := &internal.Server{
		Gatherer: gatherer,
		Port:     viper.GetInt("port"),
	}
	return server.Start()
}

func createRefreshingEcobeeClient(clientId, tokenFile string) (*ecobee.Client, error) {
	if clientId == "" {
		return nil, fmt.Errorf("ecobee client id was not defined. Please run again with ECOBEE_CLIENTID defined")
	}
	return ecobee.NewClient(clientId, tokenFile), nil
}

func createNonRefreshingEcobeeClient(tokenFile string) (*ecobee.Client, error) {
	file, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	err = json.Unmarshal(file, &token)
	if err != nil {
		return nil, err
	}

	return &ecobee.Client{
		Client: oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&token)),
	}, nil
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
	log.SetLevel(log.WarnLevel)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	rootCmd.Flags().IntP("port", "p", 9500, "Port number to listen on (env: PORT)")
	_ = viper.BindPFlag("server.port", rootCmd.Flags().Lookup("port"))

	rootCmd.Flags().String("ecobeeClientId", "", "Ecobee Client ID (ECOBEE_CLIENTID)")
	_ = viper.BindPFlag("ecobee.clientid", rootCmd.Flags().Lookup("ecobeeClientId"))

	rootCmd.Flags().Bool("handleAuthRefresh", true, "Handle OAuth token refresh internally")
	_ = viper.BindPFlag("auth.handle-refresh", rootCmd.Flags().Lookup("handleAuthRefresh"))

	rootCmd.Flags().String("auth-token-file", "auth-cache.json", "File that contains the OAuth token (AUTH_TOKEN_FILE)")
	_ = viper.BindPFlag("auth.token-file", rootCmd.Flags().Lookup("auth-token"))

	rootCmd.Flags().Bool("debug", false, "Enable debug logging (env: DEBUG)")
	_ = viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	viper.AutomaticEnv()
}
