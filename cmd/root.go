/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"ecobee-exporter/internal"
	"github.com/rspier/go-ecobee/ecobee"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ecobee-exporter",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: startServer,
}

func startServer(cmd *cobra.Command, args []string) error {
	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}
	log.Info("Welcome to Ecobee Exporter!")

	log.Info("Creating Ecobee client...")
	client := ecobee.NewClient(viper.GetString("auth.clientId"), "auth-cache.json")

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

	rootCmd.Flags().IntP("port", "p", 9500, "Port number to listen on")
	_ = viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))

	rootCmd.Flags().String("ecobeeClientId", "", "Ecobee Client ID")
	_ = viper.BindPFlag("auth.clientId", rootCmd.Flags().Lookup("ecobeeClientId"))

	rootCmd.Flags().Bool("debug", false, "Enable debug logging")
	_ = viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
}
