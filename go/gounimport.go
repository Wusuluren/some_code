package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"strings"
)

type Pos struct {
	begin token.Pos
	end   token.Pos
}
type ImportPkg struct {
	name string
	used bool
	spec *ast.ImportSpec
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	filename := flag.Args()[0]
	blob, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fileset := token.NewFileSet()
	f, err := parser.ParseFile(fileset, "", blob, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	numImportPkg := 0
	importPkgs := make([]*ImportPkg, 0, len(f.Imports))
	importPkgUsed := make(map[string]bool)
	for _, importSpec := range f.Imports {
		var pkgName string
		if importSpec.Name != nil {
			pkgName = importSpec.Name.Name
		} else {
			pkgName = importSpec.Path.Value[1: len(importSpec.Path.Value)-1]
			pkgNameSplit := strings.Split(pkgName, "/")
			pkgName = pkgNameSplit[len(pkgNameSplit)-1]
		}
		if pkgName == "_" {
			continue
		}
		importPkgUsed[pkgName] = false
		importPkgs = append(importPkgs, &ImportPkg{
			name: pkgName,
			spec: importSpec,
		})
		numImportPkg++
		fmt.Println(pkgName, importSpec)
	}
	for _, ident := range f.Unresolved {
		if _, ok := importPkgUsed[ident.Name]; ok && importPkgUsed[ident.Name] == false {
			importPkgUsed[ident.Name] = true
			numImportPkg--
			if numImportPkg <= 0 {
				break
			}
		}
	}
	if numImportPkg == 0 {
		os.Exit(0)
	}
	rmPos := make([]*Pos, 0)
	for _, importPkg := range importPkgs {
		if used, ok := importPkgUsed[importPkg.name]; ok && !used {
			var i token.Pos
			importSpec := importPkg.spec
			for i = importSpec.Pos() - 1; i < importSpec.End()-1; i++ {
				blob[i] = ' '
			}
			for i = importSpec.Pos() - 1; blob[i] != '\n'; i-- {
			}
			i += 1
			if string(blob[i:i+6]) == "import" {
				for j := i; j < i+6; j++ {
					blob[j] = ' '
				}
			}
			rmPos = append(rmPos, &Pos{
				begin: i - 1,
				end:   importSpec.End() - 1,
			})
		}
	}
	var begin token.Pos
	blobSize := len(blob)
	output := make([]byte, 0, blobSize)
	for _, pos := range rmPos {
		output = append(output, blob[begin:pos.begin]...)
		begin = pos.end
	}
	output = append(output, blob[begin:blobSize]...)
	err = ioutil.WriteFile(filename, output, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
