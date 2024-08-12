package rename

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"log"
	"path/filepath"

	"github.com/sirkon/gosrcfmt"
	"github.com/wallyqs/go-imports-rename/pkg/replacer"
)

func getFullPath(root string, name string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: full absolute path computation", err)
	}
	return filepath.Join(rootAbs, name), nil
}

func Rename(root, from, to string) error {
	var rep replacer.Replacer
	rep, err := replacer.Regexp(from, to)
	if err != nil {
		return err
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		_, base := filepath.Split(path)
		if info.IsDir() {
			if strings.HasPrefix(base, ".") && len(base) > 1 {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		goFile, err := parser.ParseFile(fset, path, nil, parser.AllErrors|parser.ParseComments)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		for _, imp := range goFile.Imports {
			pathValue := strings.Trim(imp.Path.Value, `"`)
			rep := rep.Replace(pathValue)
			switch v := rep.(type) {
			case replacer.Replacement:
				imp.Path.Value = fmt.Sprintf(`"%s"`, v.String())
			case replacer.Nothing:
				continue
			default:
				log.Printf("invalid variant case %T\n", v)
			}

			fullPath, err := getFullPath(root, info.Name())
			if err != nil {
				log.Fatalf("failed to resolve absolute path of %s", path)
			}
			dir, base := filepath.Split(fullPath)
			file, err := ioutil.TempFile(dir, base)
			if err != nil {
				return fmt.Errorf("failed to update %s", path)
			}
			formatted, err := gosrcfmt.AST(fset, goFile)
			if err != nil {
				return fmt.Errorf("error when formatting a file")
			}
			if _, err := io.Copy(file, bytes.NewBuffer(formatted)); err != nil {
				return fmt.Errorf("error when saving changes to %s", path)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("something went wrong for %s", path)
			}
			if err := os.Rename(file.Name(), path); err != nil {
				return fmt.Errorf("failed to update %s", path)
			}
		}

		return nil
	})
}
