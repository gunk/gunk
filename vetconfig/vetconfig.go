package vetconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/generate/downloader"
)

func Run(dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".gunkconfig") {
			reader, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("unable to open file: %w", err)
			}
			defer reader.Close()
			cfg, err := config.LoadSingle(reader)
			if err != nil {
				return fmt.Errorf("unable to load gunkconfig: %w", err)
			}
			vetCfg(path, cfg)
		}
		return nil
	})
	return err
}

func vetCfg(dir string, cfg *config.Config) {
	if cfg.ProtocVersion == "" {
		fmt.Printf("%s: specify protoc version\n", dir)
	}

	for _, g := range cfg.Generators {
		code := g.Code()
		if code == "ts" || code == "js" {
			if !g.FixPaths {
				fmt.Printf(
					"%s: add fix_paths_postproc=true [generate %s]\n",
					dir,
					code)
			}
		}
		if code == "grpc-gateway" {
			version := g.PluginVersion
			if version != "" {
				s := strings.Split(version, ".")
				s[0] = strings.TrimPrefix(s[0], "v")
				major, err := strconv.Atoi(s[0])
				if err != nil {
					panic(err)
				}
				if major < 2 {
					fmt.Printf(
						"%s: use new version - plugin_version=v2.3.0 [generate %s]\n",
						dir,
						code)
				}
			}
		}
		if code == "swagger" {
			fmt.Printf(
				"%s: do not use swagger. [generate %s] Use:\n[generate openapiv2]\njson_names_for_fields=true\nplugin_version=v2.3.0\n\n",
				dir,
				code)
		}
		if code == "openapiv2" {
			if _, ok := g.GetParam("json_names_for_fields"); !ok {
				fmt.Printf(
					"%s: specify json_names_for_fields=false (or true) [generate %s]\n",
					dir,
					code)
			}
		}
		if code == "go" {
			version := g.PluginVersion
			if version != "" {
				s := strings.Split(version, ".")
				if len(s) > 1 {
					minor, err := strconv.Atoi(s[1])
					if err == nil {
						if minor < 20 {
							fmt.Printf(
								"%s: use new version - plugin_version=e471641 [generate %s]\n",
								dir,
								code)
						}
					}
				}
			}
			if _, ok := g.GetParam("plugins"); ok {
				fmt.Printf(
					"%s: do not use grpc plugin. [generate %s] Use:\n[generate grpc-go]\nplugin_version=v1.1.0\n\n",
					dir,
					code)
			}
		}
		if !g.Shortened {
			if g.ProtocGen != "" {
				if config.ProtocBuiltinLanguages[g.ProtocGen] {
					fmt.Printf(
						"%s: using protoc builtin language, use shortened version [generate %s]\n",
						dir,
						g.ProtocGen)
				} else {
					fmt.Printf(
						"%s: using protoc for external binary. "+
							"Consider using shortened version  [generate %s]\n",
						dir,
						g.ProtocGen)
				}
			} else {
				if strings.HasPrefix(g.Command, "protoc-gen-") {
					fmt.Printf(
						"%s: using command- where shortened version exists. "+
							"Use shortened version [generate %s]\n",
						dir,
						code)
				}
			}
		}
		if downloader.Has(code) {
			if g.PluginVersion == "" {
				fmt.Printf(
					"%s: pin version of %s.\n",
					dir,
					code)
			}
		}
	}
}
