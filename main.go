package main

import (
	"fmt"
	"os"

	"github.com/gunk/gunk/convert"
	"github.com/gunk/gunk/dump"
	"github.com/gunk/gunk/format"
	"github.com/gunk/gunk/generate"
	"github.com/gunk/gunk/generate/downloader"
	"github.com/gunk/gunk/lint"
	"github.com/gunk/gunk/log"
	"github.com/gunk/gunk/vetconfig"
	"github.com/spf13/cobra"
)

var version = "v0.11.5"

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	app := cobra.Command{
		Use:          "gunk",
		Short:        "The modern frontend and syntax for Protocol Buffers.",
		Version:      version,
		SilenceUsage: true,
	}
	app.SetFlagErrorFunc(func(c *cobra.Command, e error) error {
		return fmt.Errorf("%v\nRun '%s --help' for usage.", e, c.CommandPath())
	})
	// versionCmd commmand
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of gundk",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stdout, "gunk", version)
		},
	}
	app.AddCommand(versionCmd)
	// generate command
	generateCmd := &cobra.Command{
		Use:   "generate [patterns]",
		Short: "Generate code from Gunk packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generate.Run("", args...)
		},
	}
	generateCmd.Flags().BoolVarP(&log.PrintCommands, "print-commands", "x", false, "Print the commands")
	generateCmd.Flags().BoolVarP(&log.Verbose, "verbose", "v", false, "Print the names of packages are they are generated")
	app.AddCommand(generateCmd)
	// convert command
	var overwrite bool
	convertCmd := &cobra.Command{
		Use:   "convert [-overwrite] [file | directory]...",
		Short: "Convert Proto file to Gunk file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return convert.Run(args, overwrite)
		},
	}
	convertCmd.Flags().BoolVarP(&overwrite, "overwrite", "w", false, "Overwrite the converted Gunk file if it exists.")
	app.AddCommand(convertCmd)
	// format command
	formatCmd := &cobra.Command{
		Use:   "format [patterns]",
		Short: "Format Gunk code",
		RunE: func(cmd *cobra.Command, args []string) error {
			return format.Run("", args...)
		},
	}
	app.AddCommand(formatCmd)
	// dump command
	var dumpFormat string
	dump := &cobra.Command{
		Use:   "dump [patterns]",
		Short: "Write a FileDescriptorSet, defined in descriptor.proto",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dump.Run(dumpFormat, "", args...)
		},
	}
	dump.Flags().StringVarP(&dumpFormat, "format", "f", "proto", "output format: [proto | json]")
	app.AddCommand(dump)
	// download list
	// TODO(hhhapz): add protoc-java, and protoc-ts, etc.
	downloadSubcommands := []func(string, string) error{
		downloadProtoc,
	}
	// download command
	downloadCmd := cobra.Command{
		Use:   "download [protoc | protoc]",
		Short: "Download the necessary tools for Gunk",
	}
	downloadCmd.Flags().BoolVarP(&log.Verbose, "verbose", "v", false, "Print details of downloaded tools")
	downloadAllCmd := cobra.Command{
		Use:   "all",
		Short: "Download all required tools for Gunk, e.g., protoc",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, f := range downloadSubcommands {
				if err := f("", ""); err != nil {
					return err
				}
			}
			return nil
		},
	}
	// download proto command
	var dlProtocPath, dlProtocVer string
	downloadProtocCmd := cobra.Command{
		Use:   "protoc",
		Short: "Download protoc",
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadProtoc(dlProtocPath, dlProtocVer)
		},
	}
	downloadProtocCmd.Flags().StringVar(&dlProtocPath, "path", "", "Path to check for protoc binary, or where to download it to")
	downloadProtocCmd.Flags().BoolVarP(&log.Verbose, "verbose", "v", false, "Print details of download tools")
	downloadProtocCmd.Flags().StringVar(&dlProtocVer, "version", "", "Version of protoc to use")
	downloadCmd.AddCommand(&downloadAllCmd, &downloadProtocCmd)
	app.AddCommand(&downloadCmd)
	// vet command
	vetCmd := cobra.Command{
		Use:   "vet [path]",
		Short: "Vet gunk config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			return vetconfig.Run(path)
		},
	}
	app.AddCommand(&vetCmd)
	// lint command
	var enableLint, disableLint string
	var listLinters bool
	lintCmd := cobra.Command{
		Use:   "lint [patterns]",
		Short: "Lint a set of Gunk files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if listLinters {
				lint.PrintLinters()
				return nil
			}
			return lint.Run("", enableLint, disableLint, args...)
		},
	}
	lintCmd.Flags().StringVar(&enableLint, "enable", "", "Linters to enable (all if empty) separated by comma")
	lintCmd.Flags().StringVar(&disableLint, "disable", "", "Linters to disable separated by comma, overrides enable")
	lintCmd.Flags().BoolVarP(&listLinters, "list", "l", false, "List all linters and exit")
	app.AddCommand(&lintCmd)
	return app.Execute()
}

func downloadProtoc(path, version string) error {
	_, err := downloader.CheckOrDownloadProtoc(path, version)
	return err
}
