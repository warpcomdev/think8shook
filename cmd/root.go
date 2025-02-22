/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"k8s.io/klog/v2"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	defer klog.Flush()
	err := CmdWebhook.Execute()
	if err != nil {
		os.Exit(1)
	}
}
