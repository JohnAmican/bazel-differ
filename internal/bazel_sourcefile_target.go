package internal

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

type BazelSourceFileTarget interface {
	Name() *string
	Digest() []byte
}

type bazelSourceFileTarget struct {
	name   *string
	digest []byte
}

func NewBazelSourceFileTarget(name string, digest []byte, workingDirectory string) (BazelSourceFileTarget, error) {
	finalDigest := bytes.NewBuffer([]byte{})
	if workingDirectory != "" && strings.HasPrefix(name, "//") {
		filenameSubstring := name[2:]
		filenamePath := strings.Replace(filenameSubstring, ":", "/", 1)
		sourceFile := path.Join(workingDirectory, filenamePath)
		if fi, err := os.Stat(sourceFile); !errors.Is(err, os.ErrNotExist) {
			// path/to/whatever does not exist
			if !fi.IsDir() {
				contents, err := os.ReadFile(sourceFile)
				if err != nil {
					return nil, fmt.Errorf("error reading file: %w", err)
				}
				finalDigest.Write(contents)
			} else {
				m, err := MD5All(sourceFile)
				if err != nil {
					return nil, fmt.Errorf("error md5'ing all files in %s: %w", sourceFile, err)
				}
				var paths []string
				for path := range m {
					paths = append(paths, path)
				}
				sort.Strings(paths)
				for _, path := range paths {
					md5 := m[path]
					finalDigest.Write(md5[:])
				}
			}
		}
	}
	finalDigest.Write(digest)
	finalDigest.Write([]byte(name))
	checksum := sha256.Sum256(finalDigest.Bytes())
	return &bazelSourceFileTarget{
		name:   &name,
		digest: checksum[:],
	}, nil
}

func (b *bazelSourceFileTarget) Name() *string {
	return b.name
}

func (b *bazelSourceFileTarget) Digest() []byte {
	return b.digest
}
