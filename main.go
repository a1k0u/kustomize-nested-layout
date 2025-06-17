package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	// TODO: replace with vanilla os?
	cp "github.com/otiai10/copy"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/kustomize/api/types"
)

var (
	flagRootDir       = flag.String("root", "", "Path to the root directory containing kustomization files")
	flagBuildDir      = flag.String("build", "", "Path for the directory where kustomize build will be executed")
	flagKustomizePath = flag.String("kustomize", "kustomize", "Path to the kustomize binary")
)

const (
	LayoutDirName = ".kustomize"
	BaseDirName   = ".base"
)

func main() {
	flag.Parse()

	if *flagRootDir == "" {
		log.Fatal("Error: --root flag is required")
	}

	if *flagBuildDir == "" {
		log.Fatal("Error: --build flag is required")
	}

	if *flagKustomizePath == "" {
		log.Fatal("Error: --kustomize flag is required")
	}

	rootDir, err := filepath.Abs(*flagRootDir)
	if err != nil {
		log.Fatalf("Error: getting absolute path for root directory: %v", err)
	}

	workDir := filepath.Join(rootDir, LayoutDirName)

	if err := os.RemoveAll(workDir); err != nil {
		log.Fatalf("Error: removing existing work directory %s: %v", workDir, err)
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		log.Fatalf("Error: creating work directory %s: %v", workDir, err)
	}

	log.Printf("Clean up and create work directory: %s", workDir)

	opt := cp.Options{
		Skip: func(info os.FileInfo, src, dest string) (bool, error) {
			return strings.HasSuffix(src, LayoutDirName), nil
		},
	}
	if err := cp.Copy(rootDir, workDir, opt); err != nil {
		log.Fatalf("Error: copying files from %s to %s: %v", rootDir, workDir, err)
	}

	log.Println("Files copied successfully")

	err = filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if filepath.Base(path) == BaseDirName {
			return nil
		}

		baseDir := filepath.Join(path, BaseDirName)

		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
		}

		files, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", path, err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			srcFilePath := filepath.Join(path, file.Name())
			destFilePath := filepath.Join(baseDir, file.Name())

			if err := os.Rename(srcFilePath, destFilePath); err != nil {
				return fmt.Errorf("failed to move file %s to %s: %w", srcFilePath, destFilePath, err)
			}
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Error: during directory walk: %v", err)
	}

	log.Println("YAML files moved to base directories successfully")

	err = filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !IsKustomizationFile(d) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read kustomization file %s: %w", path, err)
		}

		var kustomization types.Kustomization
		if err := yaml.Unmarshal(content, &kustomization); err != nil {
			return fmt.Errorf("failed to unmarshal kustomization file %s: %w", path, err)
		}

		for i, resource := range kustomization.Resources {
			if !IsDotsPath(resource) {
				continue
			}
			kustomization.Resources[i] = filepath.Join(resource, "..", BaseDirName)
		}

		updatedContent, err := yaml.Marshal(&kustomization)
		if err != nil {
			return fmt.Errorf("failed to marshal updated kustomization %s: %w", path, err)
		}

		if err := os.WriteFile(path, updatedContent, 0644); err != nil {
			return fmt.Errorf("failed to write updated kustomization %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Error: updating kustomization files: %v", err)
	}

	log.Println("Kustomization files updated successfully")

	buildDir, err := filepath.Abs(*flagBuildDir)
	if err != nil {
		log.Fatalf("Error: getting absolute path for build directory: %v", err)
	}

	isSub, buildPath, err := SubElem(rootDir, buildDir)
	if err != nil {
		log.Fatalf("Error: checking if build directory is a subdirectory of source directory: %v", err)
	}
	if !isSub {
		log.Fatalf("Error: build directory %s is not a subdirectory of root directory %s", buildDir, rootDir)
	}

	workBuildDir := filepath.Join(workDir, buildPath, BaseDirName)

	cmd := exec.Command(*flagKustomizePath, "build", workBuildDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Error: running kustomize build: %v", err)
	}
}

func IsKustomizationFile(file fs.DirEntry) bool {
	if file.IsDir() {
		return false
	}

	return slices.Contains([]string{"kustomization.yaml", "kustomization.yml", "Kustomization"}, file.Name())
}

func IsDotsPath(path string) bool {
	isDotsPath := true
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part != ".." && part != "." && part != "" {
			isDotsPath = false
			break
		}
	}
	return isDotsPath
}

func SubElem(parent, sub string) (bool, string, error) {
	up := ".." + string(filepath.Separator)

	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, "", err
	}

	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, rel, nil
	}

	return false, "", nil
}
