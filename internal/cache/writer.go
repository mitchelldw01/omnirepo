package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/mitchelldw01/omnirepo/internal/log"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Provides an io.WriteCloser that writes to the final cache destination.
// The destination can be either the file system or an S3 bucket.
type TransportWriter interface {
	Writer(path string) (io.WriteCloser, error)
}

type CacheWriter struct {
	transport TransportWriter
	reader    *CacheReader
	// The temporary directory for cache files before they're compressed
	tmpCache string
}

func NewCacheWriter(tw TransportWriter, cr *CacheReader) *CacheWriter {
	return &CacheWriter{
		transport: tw,
		reader:    cr,
		tmpCache:  filepath.Join(os.TempDir(), "omni-prev-cache"),
	}
}

func (w *CacheWriter) WriteTaskResult(dir, name string, res TaskResult) error {
	path := filepath.Join(w.tmpCache, dir, "results", name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to write task result: %v", err)
	}

	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %v", err)
	}

	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("failed to write cache result: %v", err)
	}

	return nil
}

func (w *CacheWriter) Update() error {
	if w.reader.isWorkValid && w.reader.invalidNodes.size() == 0 {
		return nil
	}

	s, err := w.startSpinner()
	if err != nil {
		return err
	}
	defer s.Stop()

	if err := w.updateWorkspace(); err != nil {
		return err
	}

	for dir, connMap := range w.reader.invalidNodes.toUnsafeMap() {
		if err := w.updateTarget(dir, connMap); err != nil {
			return err
		}
	}

	if err := w.restoreOutputs(); err != nil {
		return fmt.Errorf("failed to restore cached outputs: %v", err)
	}

	return nil
}

func (w *CacheWriter) startSpinner() (*spinner.Spinner, error) {
	fmt.Print("\n")
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Updating cache..."
	s.FinalMSG = "Cache update complete.\n"

	if !log.NoColor {
		s.FinalMSG = "✅ " + s.FinalMSG
		if err := s.Color("cyan"); err != nil {
			return nil, err
		}
	}

	s.Start()
	return s, nil
}

func (w *CacheWriter) updateWorkspace() error {
	if w.reader.isWorkValid {
		return nil
	}

	paths, err := w.getWorkspacePaths()
	if err != nil {
		return err
	}

	hashes, err := w.computeHashMap(paths)
	if err != nil {
		return err
	}

	return w.writeWorkspaceArtifacts(hashes)
}

func (w *CacheWriter) getWorkspacePaths() ([]string, error) {
	patternSet := map[string]struct{}{}
	for _, cfg := range w.reader.targetConfigs {
		for _, pattern := range cfg.WorkspaceAssets {
			patternSet[pattern] = struct{}{}
		}
	}

	patterns := []string{}
	for pattern := range patternSet {
		patterns = append(patterns, pattern)
	}

	return getCacheableWorkspacePaths(patterns, w.reader.targets)
}

func (w *CacheWriter) computeHashMap(paths []string) (map[string]struct{}, error) {
	hashes, err := w.reader.hasher.hash(paths...)
	if err != nil {
		return nil, err
	}

	hashMap := make(map[string]struct{}, len(hashes))
	for _, hash := range hashes {
		hashMap[hash] = struct{}{}
	}

	return hashMap, nil
}

func (w *CacheWriter) writeWorkspaceArtifacts(hashes map[string]struct{}) error {
	b, err := json.Marshal(hashes)
	if err != nil {
		return fmt.Errorf("failed to marshal workspace hashes: %v", err)
	}

	tw, err := w.transport.Writer("workspace.json")
	if err != nil {
		return err
	}
	defer tw.Close()

	_, err = tw.Write(b)
	return err
}

func (w *CacheWriter) updateTarget(dir string, hashes map[string]struct{}) error {
	includes := w.concatIncludesPatterns(w.reader.targetConfigs[dir], hashes)
	excludes := w.concatExcludesPatterns(w.reader.targetConfigs[dir], hashes)
	paths, err := getCacheableTargetPaths(dir, includes, excludes)
	if err != nil {
		return err
	}

	hashMap, err := w.computeHashMap(paths)
	if err != nil {
		return err
	}

	return w.writeTargetArtifacts(dir, hashMap)
}

func (w *CacheWriter) concatIncludesPatterns(cfg usercfg.TargetConfig, hashes map[string]struct{}) []string {
	patternSet := map[string]struct{}{}

	for task := range hashes {
		for _, pattern := range cfg.Pipeline[task].Includes {
			patternSet[pattern] = struct{}{}
		}
	}

	patterns := []string{}
	for pattern := range patternSet {
		patterns = append(patterns, pattern)
	}

	return patterns
}

func (w *CacheWriter) concatExcludesPatterns(cfg usercfg.TargetConfig, hashes map[string]struct{}) []string {
	patternSet := map[string]struct{}{}

	for task := range hashes {
		for _, pattern := range cfg.Pipeline[task].Excludes {
			patternSet[pattern] = struct{}{}
		}
	}

	patterns := []string{}
	for pattern := range patternSet {
		patterns = append(patterns, pattern)
	}

	return patterns
}

func (w *CacheWriter) writeTargetArtifacts(dir string, hashes map[string]struct{}) error {
	tmp := filepath.Join(w.tmpCache, dir)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return fmt.Errorf("failed to write cache artifact: %v", err)
	}
	if err := w.writeInputArtifacts(tmp, hashes); err != nil {
		return err
	}
	if err := w.writeOutputArtifacts(dir, w.reader.outputs.data[dir]); err != nil {
		return err
	}

	tw, err := w.transport.Writer(fmt.Sprintf("%s-meta.tar.zst", dir))
	if err != nil {
		return err
	}
	defer tw.Close()

	return createTarZst(tmp, tw)
}

func (w *CacheWriter) writeInputArtifacts(dir string, hashes map[string]struct{}) error {
	inputsJson, err := json.Marshal(hashes)
	if err != nil {
		return fmt.Errorf("failed to marshal target hashes %q: %v", dir, hashes)
	}

	path := filepath.Join(dir, "inputs.json")
	if err := os.WriteFile(path, inputsJson, 0o644); err != nil {
		return fmt.Errorf("failed to write cache artifact: %v", err)
	}

	return nil
}

func (w *CacheWriter) writeOutputArtifacts(dir string, patterns []string) error {
	paths, err := getCacheableOutputPaths(dir, patterns)
	if err != nil {
		return err
	}

	for _, path := range paths {
		dst := filepath.Join(w.tmpCache, dir, "outputs", w.trimTargetDirectory(path))
		if err := w.copyOutputArtifact(path, dst); err != nil {
			return err
		}
	}

	return nil
}

func (w *CacheWriter) trimTargetDirectory(path string) string {
	cleaned := filepath.Clean(path)
	trimmed := strings.TrimPrefix(cleaned, string(filepath.Separator))
	parts := strings.Split(trimmed, string(filepath.Separator))
	return filepath.Join(parts[1:]...)
}

func (w *CacheWriter) restoreOutputs() error {
	var wg sync.WaitGroup
	ch := make(chan error, 1)

	for dir, outputs := range w.reader.outputs.data {
		wg.Add(1)
		go func(dir string, outputs []string) {
			defer wg.Done()
			if err := w.restoreTargetOutputs(dir); err != nil {
				select {
				case ch <- err:
				default:
				}
			}
		}(dir, outputs)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return <-ch
}

func (w *CacheWriter) restoreTargetOutputs(dir string) error {
	src := filepath.Join(w.reader.tmpCache, dir, "outputs")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	return w.copyOutputDirectory(src, dir)
}

func (w *CacheWriter) copyOutputDirectory(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory %q: %v", dst, err)
	}

	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %q: %v", src, err)
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to determine relative file path from %q to %q: %v", src, path, err)
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("failed to create cache directory %q: %v", dstPath, err)
			}
			return nil
		}

		return w.copyOutputArtifact(path, dstPath)
	})
}

func (w *CacheWriter) copyOutputArtifact(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory %q: %v", dst, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %v", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create cache artifact %q: %v", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy %q to %q: %v", src, dst, err)
	}

	return nil
}
