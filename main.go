package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	// TODO: replace with vanilla os?
	cp "github.com/otiai10/copy"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/kustomize/api/types"
)

var (
	flagSourceDir = flag.String("source", "./kubernetes", "Path to the source directory with kustomize files")
	flagBuildDir  = flag.String("build", "", "Path to the kustomize build")
)

const (
	LayoutDirName = ".kustomize"
	BaseDirName   = ".base"
)

func main() {
	flag.Parse()

	if *flagSourceDir == "" {
		log.Fatal("Error: --source flag is required")
	}

	if *flagBuildDir == "" {
		log.Fatal("Error: --build flag is required")
	}

	sourceDir, err := filepath.Abs(*flagSourceDir)
	if err != nil {
		log.Fatalf("Error getting absolute path for source directory: %v", err)
	}

	outputDir := filepath.Join(sourceDir, LayoutDirName)

	log.Printf("Source directory: %s", sourceDir)
	log.Printf("Output directory: %s", outputDir)

	log.Printf("Cleaning up and creating output directory: %s", outputDir)
	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("Error removing existing output directory %s: %v", outputDir, err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory %s: %v", outputDir, err)
	}

	opt := cp.Options{
		Skip: func(info os.FileInfo, src, dest string) (bool, error) {
			return strings.HasSuffix(src, LayoutDirName), nil
		},
	}
	if err := cp.Copy(sourceDir, outputDir, opt); err != nil {
		log.Fatalf("Error copying files from %s to %s: %v", sourceDir, outputDir, err)
	}

	log.Println("Files copied successfully.")

	err = filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if filepath.Base(path) == BaseDirName {
			return nil
		}

		dest := filepath.Join(path, BaseDirName)

		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("failed to remove existing .base directory at %s: %w", dest, err)
		}

		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory at %s: %w", BaseDirName, dest, err)
		}

		files, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", path, err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") {
				srcFilePath := filepath.Join(path, file.Name())
				destFilePath := filepath.Join(dest, file.Name())

				if err := os.Rename(srcFilePath, destFilePath); err != nil {
					return fmt.Errorf("failed to move file %s to %s: %w", srcFilePath, destFilePath, err)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Error during directory walk: %v", err)
	}
	log.Println("YAML movement phase completed.")

	log.Println("Updating kustomization.yaml files...")
	err = filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Name() != "kustomization.yaml" && d.Name() != "kustomization.yml" {
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
			if resource == "../" {
				kustomization.Resources[i] = filepath.Join("..", "..", BaseDirName)
			}
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
		log.Fatalf("Error updating kustomization files: %v", err)
	}
	log.Println("Kustomization files updated successfully.")

	buildDir, err := filepath.Abs(*flagBuildDir)
	if err != nil {
		log.Fatalf("Error getting absolute path for build directory: %v", err)
	}

	isSub, relPath, err := SubElem(sourceDir, buildDir)
	if err != nil {
		log.Fatalf("Error checking if build directory is a subdirectory of source directory: %v", err)
	}
	if !isSub {
		log.Fatalf("Error: build directory %s is not a subdirectory of source directory %s", buildDir, sourceDir)
	}

	kustomizeRealDir := filepath.Join(outputDir, relPath, BaseDirName)
	log.Printf("Kustomize real directory: %s", kustomizeRealDir)

	cmd := exec.Command("kustomize", "build", kustomizeRealDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error running kustomize build: %v", err)
	}
	log.Println("Kustomize build output:")
	log.Println("------------------------------------")

	log.Println(string(output))
}

func SubElem(parent, sub string) (bool, string, error) {
	up := ".." + string(os.PathSeparator)

	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, "", err
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, rel, nil
	}
	return false, "", nil
}
