package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type options struct {
	RepoURL     string
	DestFolder  string
	ExcludeDirs []string
	Include     string
	Extensions  []string
	SingleFile  bool
}

func main() {
	repoURL := flag.String("repo", "", "URL of the Git repository")
	destFolder := flag.String("dest", "", "Destination folder for flattened files")
	excludeDirs := flag.String("exclude", "", "Comma-separated list of directories to exclude")
	include := flag.String("include", "", "Only include files from this directory")
	exts := flag.String("exts", "", "Comma-separated list of file extensions to include (e.g., .go,.txt)")
	singleFile := flag.Bool("single", false, "Flatten the repo into a single text file")

	flag.Parse()

	if *repoURL == "" || *destFolder == "" {
		fmt.Println("Usage: gitflat -repo <repository_url> -dest <destination_folder> [-exclude <dir1,dir2,...>] [-include <dir>] [-exts <.ext1,.ext2,...>] [-single]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	opts := &options{
		RepoURL:     *repoURL,
		DestFolder:  *destFolder,
		ExcludeDirs: strings.Fields(*excludeDirs),
		Include:     *include,
		Extensions:  strings.Fields(*exts),
		SingleFile:  *singleFile,
	}

	var err error
	if opts.SingleFile {
		err = flattenToSingleFile(opts)
	} else {
		err = flatten(opts)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if opts.SingleFile {
		fmt.Printf("Selected files from %s have been flattened to a single file in %s\n", *repoURL, *destFolder)
	} else {
		fmt.Printf("Selected files from %s have been flattened to %s\n", *repoURL, *destFolder)
	}
}

func flatten(opts *options) error {
	repo, err := git.PlainClone(opts.DestFolder, false, &git.CloneOptions{
		URL:               opts.RepoURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return fmt.Errorf("error cloning repository: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("error getting HEAD: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("error getting commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("error getting tree: %w", err)
	}

	err = processFiles(tree, opts, nil)
	if err != nil {
		return fmt.Errorf("error processing files: %w", err)
	}

	err = cleanupDirectories(opts.DestFolder)
	if err != nil {
		return fmt.Errorf("error removing directories: %w", err)
	}

	return nil
}

func flattenToSingleFile(opts *options) error {
	repo, err := git.PlainClone(opts.DestFolder, false, &git.CloneOptions{
		URL:               opts.RepoURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return fmt.Errorf("error cloning repository: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("error getting HEAD: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("error getting commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("error getting tree: %w", err)
	}

	outputFile, err := os.Create(filepath.Join(opts.DestFolder, "flattened_repo.txt"))
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outputFile.Close()

	err = processFiles(tree, opts, outputFile)
	if err != nil {
		return fmt.Errorf("error processing files: %w", err)
	}

	err = cleanupDirectories(opts.DestFolder)
	if err != nil {
		return fmt.Errorf("error cleaning up directory: %w", err)
	}

	return nil
}

func processFiles(tree *object.Tree, opts *options, outputWriter io.Writer) error {
	return tree.Files().ForEach(func(f *object.File) error {
		if shouldExclude(f.Name, opts.ExcludeDirs, opts.Include) {
			return nil
		}

		if !hasValidExtension(f.Name, opts.Extensions) {
			return nil
		}

		content, err := f.Contents()
		if err != nil {
			return fmt.Errorf("error reading file contents: %w", err)
		}

		if opts.SingleFile {
			_, err = fmt.Fprintf(outputWriter, "--- %s ---\n%s\n\n", f.Name, content)
		} else {
			targetPath := filepath.Join(opts.DestFolder, filepath.Base(f.Name))
			err = os.WriteFile(targetPath, []byte(content), 0644)
		}
		if err != nil {
			return fmt.Errorf("error writing file: %w", err)
		}
		return nil
	})
}

func cleanupDirectories(destFolder string) error {
	return filepath.Walk(destFolder, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path != destFolder && info.IsDir() {
			err := os.RemoveAll(path)
			if err != nil {
				return fmt.Errorf("error removing directory: %w", err)
			}
			return filepath.SkipDir
		}
		return nil
	})
}

func shouldExclude(path string, excludeDirs []string, include string) bool {
	if include != "" {
		return !strings.HasPrefix(path, include)
	}
	for _, dir := range excludeDirs {
		if dir != "" && strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

func hasValidExtension(path string, extensions []string) bool {
	if len(extensions) == 0 || (len(extensions) == 1 && extensions[0] == "") {
		return true
	}
	for _, validExt := range extensions {
		if validExt != "" && strings.HasSuffix(path, validExt) {
			return true
		}
	}
	return false
}
