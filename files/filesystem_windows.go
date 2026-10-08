//go:build windows

package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrInvalidAccess = errors.New("invalid access")
var ErrSymlinkNotSupported = errors.New("symlinks are not supported")

type FileServer interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS

	Prefix() string
	GetRootFD() int

	Stat(name string) (fs.FileInfo, error)
	Mkdir(path string, mode os.FileMode) error
	MkdirAll(path string, mode os.FileMode) error
	OpenFile(path string, flags int, mode os.FileMode) (*os.File, error)
	Remove(path string) error
	Rename(source, target string) error
	RemoveAll(path string) error
	Glob(pattern string) ([]string, error)
	Symlink(oldname, newname string) error
	Lstat(name string) (fs.FileInfo, error)
	Chmod(path string, mode os.FileMode) error

	Create(name string) (*os.File, error)
	Copy(sourceFS FileServer, source, target string) error

	WriteFile(name string, content []byte, mode os.FileMode) error
	CreateTemp(dir, pattern string) (*os.File, error)
	ReadFile(name string) ([]byte, error)

	Close() error
}

type fileServer struct {
	dir  string
	root *os.File

	symlinkSupport bool
	uid            int
	gid            int
}

func NewFileServer(prefix string, uid, gid int, symlinkSupport bool) (FileServer, error) {
	if prefix == "" {
		prefix = "."
	}
	f := &fileServer{dir: filepath.Clean(prefix), uid: uid, gid: gid, symlinkSupport: symlinkSupport}
	var err error
	f.root, err = f.resolveRootFd()
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (sfp *fileServer) GetRootFD() int {
	return int(sfp.root.Fd())
}

func (sfp *fileServer) Prefix() string {
	return sfp.dir
}

func (sfp *fileServer) Open(name string) (fs.File, error) {
	return sfp.OpenFile(name, os.O_RDONLY, 0644)
}

func (sfp *fileServer) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(filepath.Join(sfp.dir, prepPath(name)))
}

func (sfp *fileServer) Symlink(oldpath, newpath string) error {
	return ErrSymlinkNotSupported
}

func (sfp *fileServer) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(filepath.Join(sfp.dir, prepPath(name)))
}

func (sfp *fileServer) Glob(pattern string) ([]string, error) {
	parent := filepath.Base(pattern)
	if parent == pattern {
		parent = "."
	}
	files, err := sfp.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	results := make([]string, 0)
	for _, v := range files {
		if matches, _ := filepath.Match(pattern, v.Name()); matches {
			results = append(results, v.Name())
		}
	}
	return results, nil
}

func (sfp *fileServer) Close() error {
	return sfp.root.Close()
}

func (sfp *fileServer) OpenFile(path string, flags int, mode os.FileMode) (*os.File, error) {
	path = prepPath(path)
	if path == "" {
		return os.Open(sfp.dir)
	}
	if flags&os.O_CREATE == 0 {
		mode = 0
	}
	return os.OpenFile(filepath.Join(sfp.dir, filepath.FromSlash(path)), flags, mode)
}

func (sfp *fileServer) MkdirAll(path string, mode os.FileMode) error {
	path = prepPath(path)
	if path == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(sfp.dir, filepath.FromSlash(path)), mode)
}

func (sfp *fileServer) Rename(source, target string) error {
	source = prepPath(source)
	target = prepPath(target)
	return os.Rename(filepath.Join(sfp.dir, filepath.FromSlash(source)), filepath.Join(sfp.dir, filepath.FromSlash(target)))
}

func (sfp *fileServer) Mkdir(path string, mode os.FileMode) error {
	path = prepPath(path)
	if path == "" {
		return nil
	}
	return os.Mkdir(filepath.Join(sfp.dir, filepath.FromSlash(path)), mode)
}

func (sfp *fileServer) Remove(path string) error {
	path = prepPath(path)
	if path == "" {
		return nil
	}
	return os.Remove(filepath.Join(sfp.dir, filepath.FromSlash(path)))
}

func (sfp *fileServer) RemoveAll(path string) (err error) {
	defer func() {
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	}()
	path = prepPath(path)
	if path == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(sfp.dir, filepath.FromSlash(path)))
}

func (sfp *fileServer) Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(filepath.Join(sfp.dir, prepPath(path)))
}

func (sfp *fileServer) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(filepath.Join(sfp.dir, prepPath(path)), mode)
}

func (sfp *fileServer) Copy(sourceFileServer FileServer, sourcePath, targetPath string) error {
	parent := filepath.Dir(filepath.Clean(targetPath))
	if parent != "/" && parent != "." && parent != "" {
		_ = sfp.MkdirAll(parent, 0755)
	}

	sourceFile, err := sourceFileServer.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := sfp.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	return err
}

func (sfp *fileServer) Create(name string) (*os.File, error) {
	return sfp.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
}

func (sfp *fileServer) WriteFile(name string, content []byte, mode os.FileMode) error {
	f, err := sfp.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	if content != nil {
		_, err = f.Write(content)
	}
	return err
}

func (sfp *fileServer) CreateTemp(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

func (sfp *fileServer) ReadFile(name string) ([]byte, error) {
	f, err := sfp.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func prepPath(path string) string {
	path = filepath.Clean(filepath.FromSlash(path))
	path = strings.TrimPrefix(path, string(filepath.Separator))
	if path == "." || path == string(filepath.Separator) {
		return ""
	}
	return path
}

func (sfp *fileServer) resolveRootFd() (*os.File, error) {
	return os.Open(sfp.dir)
}
