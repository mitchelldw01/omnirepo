package cache

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func getCacheableWorkspacePaths(includes, targets []string) ([]string, error) {
	paths := []string{}
	return paths, filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk workspace directory: %v", err)
		}
		if slices.Contains(targets, path) {
			return nil
		}

		isMatch, err := checkForMatch(path, includes)
		if err != nil {
			return err
		}
		if isMatch {
			paths = append(paths, path)
		}

		return nil
	})
}

func getCacheableTargetPaths(dir string, includes, excludes []string) ([]string, error) {
	paths := []string{}
	return paths, filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %q: %v", dir, err)
		}

		normalized := filepath.Join(strings.Split(path, string(filepath.Separator))[1:]...)
		isMatch, err := checkForMatch(normalized, excludes)
		if err != nil {
			return err
		}
		if isMatch {
			return nil
		}

		isMatch, err = checkForMatch(normalized, includes)
		if err != nil {
			return err
		}
		if isMatch {
			paths = append(paths, path)
		}

		return nil
	})
}

func getCacheableOutputPaths(dir string, patterns []string) ([]string, error) {
	paths := []string{}
	return paths, filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %q: %v", dir, err)
		}

		normalized := filepath.Join(strings.Split(path, string(filepath.Separator))[1:]...)
		isMatch, err := checkForMatch(normalized, patterns)
		if err != nil {
			return err
		}
		if isMatch {
			paths = append(paths, path)
		}

		return nil
	})
}

func checkForMatch(path string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		isMatch, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, fmt.Errorf("failed to match path %q again pattern %q: %v", path, pattern, err)
		}
		if isMatch {
			return true, nil
		}
	}

	return false, nil
}
