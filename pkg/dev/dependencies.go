package dev

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/bundlediscover"
	"github.com/flynn/go-shlex"
	"github.com/pkg/errors"
)

func InstallBundleDependencies(bundle bundlediscover.Bundle) error {
	instructions, err := build.GetBundleBuildInstructions(build.BundleDockerfileConfig{
		Root: bundle.RootPath,
		Options: build.KindOptions{
			"shim": "true",
		},
		BuildContext: bundle.BuildContext,
	})
	if err != nil {
		if _, ok := errors.Cause(err).(build.ErrUnsupportedBuilder); ok {
			return nil
		}
		return err
	}

	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString("set -xeo pipefail\n")

	for _, inst := range instructions.InstallInstructions {
		if inst.SrcPath != "" && inst.DstPath != "" && inst.DstPath != "." {
			// Weed out things that don't exist. cp errors if you try to pass it something that
			// doesn't exist. Use filepath.Glob instead of os.Stat because some of these might need
			// expansion.
			srcs, err := shlex.Split(inst.SrcPath)
			if err != nil {
				return err
			}
			existingSrcs := []string{}
			for _, src := range srcs {
				if matches, err := filepath.Glob(path.Join(bundle.RootPath, src)); err != nil {
					return err
				} else if len(matches) > 0 {
					existingSrcs = append(existingSrcs, src)
				}
			}
			if len(existingSrcs) > 0 {
				srcPath := strings.Join(existingSrcs, " ")
				b.WriteString(fmt.Sprintf("cp -rn %s %s\n", srcPath, inst.DstPath))

				// Make it executable
				if inst.Executable {
					b.WriteString(fmt.Sprintf("chmod +x %s\n", inst.DstPath))
				}
			}
		}
		if inst.Cmd != "" {
			b.WriteString(inst.Cmd)
			b.WriteString("\n")
		}
	}

	contents := b.String()
	if err := os.WriteFile(path.Join(bundle.RootPath, "airplane_build.sh"), []byte(contents), 0777); err != nil {
		return err
	}
	defer os.Remove(path.Join(bundle.RootPath, "airplane_build.sh"))

	cmd := exec.Command("./airplane_build.sh")
	cmd.Dir = bundle.RootPath
	for key, envVar := range bundle.BuildContext.EnvVars {
		// TODO: handle env-from-config
		if envVar.Value != nil {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, *envVar.Value))
		}
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		return errors.Wrapf(err, "error running build")
	}

	return nil
}