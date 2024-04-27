package cache

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

func unpackTarZst(src io.Reader, dst string) error {
	decoder, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	r := tar.NewReader(decoder)
	return processTarEntries(r, dst)
}

func processTarEntries(r *tar.Reader, dst string) error {
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %v", err)
		}
		if err := processTarEntry(r, header, dst); err != nil {
			return err
		}
	}

	return nil
}

func processTarEntry(r *tar.Reader, header *tar.Header, dst string) error {
	path := filepath.Join(dst, header.Name)
	perm := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("failed to create directory %q: %v", path, err)
		}
	case tar.TypeReg:
		file, err := os.OpenFile(path, perm, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("failed to open file for writing %q: %v", path, err)
		}
		defer file.Close()
		if _, err = io.Copy(file, r); err != nil {
			return fmt.Errorf("failed to write file %q: %v", path, err)
		}
	}

	return nil
}

// func createTarZst(src string, dst io.Writer) error {
// 	w, err := zstd.NewWriter(dst)
// 	if err != nil {
// 		return fmt.Errorf("failed to create zstd writer for %q: %v", dst, err)
// 	}
// 	defer w.Close()

// 	tarWriter := tar.NewWriter(w)
// 	defer tarWriter.Close()

// 	return tarDirectory(src, tarWriter)
// }

// func tarDirectory(src string, w *tar.Writer) error {
// 	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return fmt.Errorf("failed to walk directory %q: %v", src, err)
// 		}
// 		return processEntry(src, path, info, w)
// 	})
// }

// func processEntry(base, path string, info os.FileInfo, w *tar.Writer) error {
// 	header, err := tar.FileInfoHeader(info, "")
// 	if err != nil {
// 		return fmt.Errorf("failed to get header info: %v", err)
// 	}

// 	rel, err := filepath.Rel(base, path)
// 	if err != nil {
// 		return fmt.Errorf("failed to determine relative file path from %q to %q: %v", base, path, err)
// 	}
// 	header.Name = filepath.ToSlash(rel)

// 	if err := w.WriteHeader(header); err != nil {
// 		return fmt.Errorf("failed to write header: %v", err)
// 	}

// 	if !info.IsDir() {
// 		return writeFileToTar(path, w)
// 	}
// 	return nil
// }

// func writeFileToTar(path string, w *tar.Writer) error {
// 	file, err := os.Open(path)
// 	if err != nil {
// 		return fmt.Errorf("error opening file: %w", err)
// 	}
// 	defer file.Close()

// 	if _, err = io.Copy(w, file); err != nil {
// 		return fmt.Errorf("failed to write file %q: %v", path, err)
// 	}
// 	return nil
// }
