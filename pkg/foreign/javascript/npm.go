package javascript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type PackageManifest struct {
	Exports map[string]string `json:"exports"`
	Path    string            `json:"path"`
}

func InstallGlobalPackage(packageName string) error {
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command("npm", "install", "-g", packageName)
	cmd.Stdout = nil
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed installing package '%s' with NPM: %w %s", packageName, err, stderr.String())
	}
	return nil
}

func ResolveGlobalPackage(packageName string, orInstall bool) (manifest PackageManifest, err error) {
	for {
		type lsResponse struct {
			Dependencies map[string]PackageManifest `json:"dependencies"`
		}
		stdout := bytes.NewBuffer(nil)
		cmd := exec.Command("npm", "ls", packageName, "-g", "-json", "-long")
		stderr := strings.Builder{}
		cmd.Stdout = stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			err = fmt.Errorf("failed resolving package '%s' with NPM: %w %s", packageName, err, stderr.String())
			return
		}
		var res lsResponse
		err = json.Unmarshal(stdout.Bytes(), &res)
		if err != nil {
			err = fmt.Errorf("failed resolving package '%s' with NPM: %w", packageName, err)
			return
		}

		for k, dep := range res.Dependencies {
			if k == packageName {
				return dep, nil
			}
		}

		if orInstall && InstallGlobalPackage(packageName) == nil {
			orInstall = false
			continue
		}
		break
	}
	err = fmt.Errorf("package '%s' is not installed globally", packageName)
	return
}
func ResolveGlobalImport(importPath string, orInstall bool) (string, error) {
	packageName, path, found := strings.Cut(importPath, "/")
	if !found {
		path = "."
	} else {
		path = "./" + path
	}

	manifest, err := ResolveGlobalPackage(packageName, orInstall)
	if err != nil {
		return "", err
	}

	rel, ok := manifest.Exports[path]
	if !ok {
		rel = path
	}
	return filepath.Join(manifest.Path, rel), nil
}
