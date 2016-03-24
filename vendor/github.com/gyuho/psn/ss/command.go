package ss

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type Flags struct {
	LogPath string

	Top    int
	Filter Process

	Kill    bool
	CleanUp bool

	Monitor         bool
	MonitorInterval time.Duration
}

var (
	Command = &cobra.Command{
		Use:   "ss",
		Short: "Investigates sockets.",
		RunE:  CommandFunc,
	}
	KillCommand = &cobra.Command{
		Use:   "ss-kill",
		Short: "Kills sockets.",
		RunE:  KillCommandFunc,
	}
	MonitorCommand = &cobra.Command{
		Use:   "ss-monitor",
		Short: "Monitors sockets.",
		RunE:  MonitorCommandFunc,
	}
	cmdFlag = Flags{}
)

func init() {
	Command.PersistentFlags().StringVarP(&cmdFlag.Filter.Program, "program", "s", "", "Specify the program. Empty lists all programs.")
	Command.PersistentFlags().StringVarP(&cmdFlag.Filter.LocalPort, "local-port", "l", "", "Specify the local port. Empty lists all local ports.")
	Command.PersistentFlags().IntVarP(&cmdFlag.Top, "top", "t", 0, "Only list the top processes (descending order in memory usage). 0 means all.")

	KillCommand.PersistentFlags().StringVarP(&cmdFlag.Filter.Program, "program", "s", "", "Specify the program. Empty lists all programs.")
	KillCommand.PersistentFlags().StringVarP(&cmdFlag.Filter.LocalPort, "local-port", "l", "", "Specify the local port. Empty lists all local ports.")
	KillCommand.PersistentFlags().BoolVarP(&cmdFlag.CleanUp, "clean-up", "c", false, "'true' to automatically kill deleted processes. Name must be empty.")

	MonitorCommand.PersistentFlags().StringVarP(&cmdFlag.Filter.Program, "program", "s", "", "Specify the program. Empty lists all programs.")
	MonitorCommand.PersistentFlags().StringVarP(&cmdFlag.Filter.LocalPort, "local-port", "l", "", "Specify the local port. Empty lists all local ports.")
	MonitorCommand.PersistentFlags().IntVarP(&cmdFlag.Top, "top", "t", 0, "Only list the top processes (descending order in memory usage). 0 means all.")
	MonitorCommand.PersistentFlags().StringVar(&cmdFlag.LogPath, "log-path", "", "File path to store logs. Empty to print out to stdout.")
	MonitorCommand.PersistentFlags().DurationVar(&cmdFlag.MonitorInterval, "monitor-interval", 10*time.Second, "Monitor interval.")
}

func CommandFunc(cmd *cobra.Command, args []string) error {
	color.Set(color.FgMagenta)
	fmt.Fprintf(os.Stdout, "\npsn ss\n\n")
	color.Unset()

	ssr, err := List(&cmdFlag.Filter, TCP, TCP6)
	if err != nil {
		return err
	}
	WriteToTable(os.Stdout, cmdFlag.Top, ssr...)

	color.Set(color.FgGreen)
	fmt.Fprintf(os.Stdout, "\nDone.\n")
	color.Unset()

	return nil
}

func KillCommandFunc(cmd *cobra.Command, args []string) error {
	color.Set(color.FgRed)
	fmt.Fprintf(os.Stdout, "\npsn ss-kill\n\n")
	color.Unset()

	if cmdFlag.CleanUp && cmdFlag.Filter.Program == "" {
		cmdFlag.Filter.Program = "deleted)"
	} else if cmdFlag.Filter.LocalPort == "" && cmdFlag.Filter.Program == "" { // to prevent killing all
		cmdFlag.Filter.Program = "SPECIFY PROGRAM NAME"
	}

	ssr, err := List(&cmdFlag.Filter, TCP, TCP6)
	if err != nil {
		fmt.Fprintf(os.Stdout, "\nerror (%v)\n", err)
		return nil
	}
	WriteToTable(os.Stdout, cmdFlag.Top, ssr...)
	Kill(os.Stdout, ssr...)

	color.Set(color.FgGreen)
	fmt.Fprintf(os.Stdout, "\nDone.\n")
	color.Unset()

	return nil
}

func MonitorCommandFunc(cmd *cobra.Command, args []string) error {
	color.Set(color.FgBlue)
	fmt.Fprintf(os.Stdout, "\npsn ss-monitor\n\n")
	color.Unset()

	rFunc := func() error {
		ssr, err := List(&cmdFlag.Filter, TCP, TCP6)
		if err != nil {
			return err
		}

		if filepath.Ext(cmdFlag.LogPath) == ".csv" {
			f, err := openToAppend(cmdFlag.LogPath)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := WriteToCSV(f, ssr...); err != nil {
				return err
			}
			return err
		}

		var wr io.Writer
		if cmdFlag.LogPath == "" {
			wr = os.Stdout
		} else {
			f, err := openToAppend(cmdFlag.LogPath)
			if err != nil {
				return err
			}
			defer f.Close()
			wr = f
		}
		WriteToTable(wr, cmdFlag.Top, ssr...)
		return nil
	}

	if err := rFunc(); err != nil {
		return err
	}

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
escape:
	for {
		select {
		case <-time.After(cmdFlag.MonitorInterval):
			if err := rFunc(); err != nil {
				fmt.Fprintf(os.Stdout, "error: %v\n", err)
				break escape
			}
		case sig := <-notifier:
			fmt.Fprintf(os.Stdout, "Received %v\n", sig)
			return nil
		}
	}

	color.Set(color.FgGreen)
	fmt.Fprintf(os.Stdout, "\nDone.\n")
	color.Unset()

	return nil
}
