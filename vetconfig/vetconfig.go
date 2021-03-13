package vetconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/generate/downloader"
)

var RecommendStrip = false

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
	if RecommendStrip {
		if cfg.StripEnumTypeNames == false {
			fmt.Printf("%s: use strip_enum_type_names = true\n", dir)
		}
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
