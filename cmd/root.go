package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hyejoo",
	Short: "Hyejoo discord trading card game",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
