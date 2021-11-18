package main

import (
	"fmt"
	"os"

	"github.com/gunk/gunk/convert"
	"github.com/gunk/gunk/dump"
	"github.com/gunk/gunk/format"
	"github.com/gunk/gunk/generate"
	"github.com/gunk/gunk/generate/downloader"
	"github.com/gunk/gunk/log"
	"github.com/gunk/gunk/vetconfig"
	"github.com/spf13/cobra"
)

var version = "v0.8.7"

func main() {
	os.Exit(run())
}

func run() int {
	app := cobra.Command{
		Use:     "gunk",
		Short:   "The modern frontend and syntax for Protocol Buffers.",
		Version: version,
	}
	// version commmand
	ver := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of gundk",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stdout, "gunk", version)
		},
	}
	app.AddCommand(ver)
	// generate command
	gen := &cobra.Command{
		Use:   "generate [patterns]",
		Short: "Generate code from Gunk packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generate.Run("", args...)
		},
	}
	gen.Flags().BoolVarP(&log.PrintCommands, "print-commands", "x", false, "Print the commands")
	gen.Flags().BoolVarP(&log.Verbose, "verbose", "v", false, "Print the names of packages are they are generated")
	app.AddCommand(gen)
	// convert command
	var overwrite bool
	conv := &cobra.Command{
		Use:   "convert [-overwrite] [file | directory]...",
		Short: "Convert Proto file to Gunk file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return convert.Run(args, overwrite)
		},
	}
	conv.Flags().BoolVarP(&overwrite, "overwrite", "w", false, "Overwrite the converted Gunk file if it exists.")
	// format command
	app.AddCommand(conv)
	frmt := &cobra.Command{
		Use:   "format [patterns]",
		Short: "Format Gunk code",
		RunE: func(cmd *cobra.Command, args []string) error {
			return format.Run("", args...)
		},
	}
	app.AddCommand(frmt)
	// dump command
	var dmpFormat string
	dmp := &cobra.Command{
		Use:   "dump [patterns]",
		Short: "Write a FileDescriptorSet, defined in descriptor.proto",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dump.Run(dmpFormat, "", args...)
		},
	}
	dmp.Flags().StringVarP(&dmpFormat, "format", "f", "proto", "output format: [proto | json]")
	app.AddCommand(dmp)
	// download list
	// TODO(hhhapz): add protoc-java, and protoc-ts, etc.
	downloadSubcommands := []func() error{
		func() error { return downloadProtoc("", "") },
	}
	// download command
	download := cobra.Command{
		Use:   "download [protoc | protoc]",
		Short: "Download the necessary tools for Gunk",
	}
	download.Flags().BoolVarP(&log.Verbose, "verbose", "v", false, "Print details of downloaded tools")
	dlAll := cobra.Command{
		Use:   "all",
		Short: "Download all required tools for Gunk, e.g., protoc",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, f := range downloadSubcommands {
				if err := f(); err != nil {
					return err
				}
			}
			return nil
		},
	}
	var dlProtocPath, dlProtocVer string
	dlProtoc := cobra.Command{
		Use:   "protoc",
		Short: "Download protoc",
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadProtoc(dlProtocPath, dlProtocVer)
		},
	}
	dlProtoc.Flags().StringVar(&dlProtocPath, "path", "", "Path to check for protoc binary, or where to download it to")
	dlProtoc.Flags().BoolVarP(&log.Verbose, "verbose", "v", false, "Print details of download tools")
	dlProtoc.Flags().StringVar(&dlProtocVer, "version", "", "Version of protoc to use")
	download.AddCommand(&dlAll, &dlProtoc)
	app.AddCommand(&download)
	// vet command
	vet := cobra.Command{
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
	app.AddCommand(&vet)
	// run app
	if err := app.Execute(); err != nil {
		return 1
	}
	return 0
}

func downloadProtoc(path, version string) error {
	_, err := downloader.CheckOrDownloadProtoc(path, version)
	return err
}
