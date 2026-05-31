package goway

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// scannedFile is a single candidate migration file discovered by a source,
// before its name has been parsed and validated.
type scannedFile struct {
	// name is the base file name, used for parsing and history records.
	name string

	// location is a human readable identifier of where the file was found, used
	// in diagnostics.
	location string

	// read returns the raw file content.
	read func() ([]byte, error)
}

// migrationSource enumerates candidate migration files from one location.
type migrationSource interface {
	scan() ([]scannedFile, error)
}

// parseLocation builds a filesystem backed source from a location string. The
// "filesystem:" and "classpath:" prefixes are both accepted; the latter is
// treated as a path relative to the working directory, since Go has no class
// path. A bare path is used as is.
func parseLocation(location string) migrationSource {
	switch {
	case strings.HasPrefix(location, "filesystem:"):
		return &directorySource{root: strings.TrimPrefix(location, "filesystem:")}
	case strings.HasPrefix(location, "classpath:"):
		return &directorySource{root: strings.TrimPrefix(location, "classpath:")}
	default:
		return &directorySource{root: location}
	}
}

// directorySource scans an operating system directory tree for migration files.
type directorySource struct {
	root string
}

func (s *directorySource) scan() ([]scannedFile, error) {
	info, err := os.Stat(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			// A missing location is tolerated so that callers can configure several
			// optional locations.
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var files []scannedFile
	walkErr := filepath.WalkDir(s.root, func(walkPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		captured := walkPath
		files = append(files, scannedFile{
			name:     entry.Name(),
			location: captured,
			read: func() ([]byte, error) {
				return os.ReadFile(captured)
			},
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}

// fsSource scans an fs.FS, such as one produced by go:embed, rooted at a
// directory path.
type fsSource struct {
	fileSystem fs.FS
	root       string
}

func (s *fsSource) scan() ([]scannedFile, error) {
	root := s.root
	if root == "" {
		root = "."
	}

	if _, err := fs.Stat(s.fileSystem, root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []scannedFile
	walkErr := fs.WalkDir(s.fileSystem, root, func(walkPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		captured := walkPath
		files = append(files, scannedFile{
			name:     path.Base(walkPath),
			location: captured,
			read: func() ([]byte, error) {
				return fs.ReadFile(s.fileSystem, captured)
			},
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}
