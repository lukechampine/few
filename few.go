package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
)

func main() {
	typeNames := flag.String("type", "", "comma-separated list of type names; must be set")
	flag.Parse()

	file := flag.Args()[0]

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		log.Fatal(err)
	}

	// Type-check the package.
	// We create an empty map for each kind of input
	// we're interested in, and Check populates them.
	info := types.Info{
		Defs: make(map[*ast.Ident]types.Object),
	}
	var conf types.Config
	conf.Importer = importer.Default()
	_, err = conf.Check("", fset, []*ast.File{f}, &info)
	if err != nil {
		log.Fatal(err)
	}

	for _, obj := range info.Defs {
		tn, ok := obj.(*types.TypeName)
		if !ok || tn.Name() != *typeNames {
			continue
		}
		g := generator{loopVar: 'i'}
		g.Printf(`package %s

import (
	"io"
	"reflect"
	"unsafe"
)`, "main")
		g.Printf(`
func (x *%s) WriteTo(w io.Writer) (total int64, err error) {
	var n int
	var ptrTrue = true
	var ptrFalse = false
	var sli *reflect.SliceHeader
	var str *reflect.StringHeader
	_, _, _, _, _ = n, ptrTrue, ptrFalse, sli, str // evade unused variable check
`, tn.Name())
		g.generate("(*x)", tn.Type())
		g.Printf(`
	return
}`)
		src, err := format.Source(g.buf.Bytes())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(src))
		return
	}
	fmt.Println("not found")
	return
}
