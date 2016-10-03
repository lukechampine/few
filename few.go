package main

import (
	"bytes"
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
	var n int`, tn.Name())
		g.generate("x", tn.Type())
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

type generator struct {
	buf     bytes.Buffer
	loopVar byte // i, j, k, etc.
}

func (g *generator) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format, args...)
}

func (g *generator) generate(name string, t types.Type) {
	switch t := t.Underlying().(type) {
	case *types.Basic:
		if t.Kind() == types.String {
			// unlike other basic types, strings have a pointer indirection
			g.generateString(name)
		} else {
			g.generateBasic(name)
		}
	case *types.Pointer:
		g.generatePointer(name, t)
	case *types.Slice:
		g.generateLoop(name, t.Elem())
	case *types.Array:
		g.generateLoop(name, t.Elem())
	case *types.Struct:
		g.generateStruct(name, t)

	}
}

const writeTempl = `
	n, err = w.Write(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: %[1]s, Len: %[2]s, Cap: %[2]s})))
	if err != nil {
		return
	}
	total += int64(n)
`

func (g *generator) generateBasic(name string) {
	ptr := fmt.Sprintf("uintptr(unsafe.Pointer(&%s))", name)
	size := fmt.Sprintf("int(unsafe.Sizeof(%s))", name)
	g.Printf(writeTempl, ptr, size)
}

func (g *generator) generatePointer(name string, t *types.Pointer) {
	// TODO: eliminate &*
	g.generate("(*"+name+")", t.Elem())
}

func (g *generator) generateString(name string) {
	ptr := fmt.Sprintf("(*reflect.StringHeader)(unsafe.Pointer(&%s)).Data", name)
	size := fmt.Sprintf("len(%s)", name) // NOTE: len appears to be just as fast as accessing StringHeader.Len directly
	g.Printf(writeTempl, ptr, size)
}

func (g *generator) generateLoop(name string, elem types.Type) {
	// TODO: if basic elem, cast array data directly
	// (short term, just naively recurse)
	g.Printf("\nfor %c := range %s {", g.loopVar, name)
	innerName := fmt.Sprintf("%s[%c]", name, g.loopVar)
	g.loopVar++ // increment loopVar for each nested loop
	g.generate(innerName, elem)
	g.loopVar--
	g.Printf("}\n")
}

func (g *generator) generateStruct(name string, st *types.Struct) {
	// TODO: if all fields are basic elems, cast struct data directly
	// (short term, just naively recurse)
	for i := 0; i < st.NumFields(); i++ {
		g.generate(name+"."+st.Field(i).Name(), st.Field(i).Type())
	}
}
