package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func main() {
	repoURL := flag.String("repo", "", "URL of the Git repository")
	destFolder := flag.String("dest", "", "Destination folder for flattened files")
	excludeDirs := flag.String("exclude", "", "Comma-separated list of directories to exclude")
	extensions := flag.String("ext", "", "Comma-separated list of file extensions to include (e.g., .go,.txt)")

	flag.Parse()

	if *repoURL == "" || *destFolder == "" {
		fmt.Println("Usage: gitflat -repo <repository_url> -dest <destination_folder> [-exclude <dir1,dir2,...>] [-ext <.ext1,.ext2,...>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	err := flatten(&options{
		RepoURL:     *repoURL,
		DestFolder:  *destFolder,
		ExcludeDirs: strings.Split(*excludeDirs, ","),
		Extensions:  strings.Split(*extensions, ","),
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Selected files from %s have been flattened to %s\n", *repoURL, *destFolder)
}

type options struct {
	RepoURL     string
	DestFolder  string
	ExcludeDirs []string
	Extensions  []string
}

func flatten(opts *options) error {
	repo, err := git.PlainClone(opts.DestFolder, false, &git.CloneOptions{
		URL:               opts.RepoURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return fmt.Errorf("error cloning repository: %v", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("error getting HEAD: %v", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("error getting commit: %v", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("error getting tree: %v", err)
	}

	err = tree.Files().ForEach(func(f *object.File) error {
		if shouldExclude(f.Name, opts.ExcludeDirs) {
			return nil
		}

		if !hasValidExtension(f.Name, opts.Extensions) {
			return nil
		}

		content, err := f.Contents()
		if err != nil {
			return err
		}

		targetPath := filepath.Join(opts.DestFolder, filepath.Base(f.Name))

		err = os.WriteFile(targetPath, []byte(content), 0644)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error processing files: %v", err)
	}

	err = filepath.Walk(opts.DestFolder, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != opts.DestFolder {
			os.RemoveAll(path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error removing directories: %v", err)
	}

	return nil
}

func shouldExclude(path string, excludeDirs []string) bool {
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
	ext := filepath.Ext(path)
	for _, validExt := range extensions {
		if validExt != "" && ext == validExt {
			return true
		}
	}
	return false
}
