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

// isContiguous returns true if all of t's data (at least, the data that we
// care about) lies in one contiguous segment of memory. This includes all
// basic types (except strings), arrays of contiguous types, and structs
// containing only contiguous types.
func isContiguous(t types.Type) bool {
	switch t := t.Underlying().(type) {
	case *types.Basic:
		return t.Kind() != types.String
	case *types.Array:
		return isContiguous(t.Elem())
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if !isContiguous(t.Field(i).Type()) {
				return false
			}
		}
		return true
	}
	return false
}

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
	case *types.Array:
		g.generateArray(name, t)
	case *types.Struct:
		g.generateStruct(name, t)
	case *types.Pointer:
		g.generatePointer(name, t)
	case *types.Slice:
		g.generateSlice(name, t)
	default:
		panic("unrecognized type: " + t.String())
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

func (g *generator) generateArray(name string, t *types.Array) {
	if t.Len() == 0 {
		return
	}
	if isContiguous(t) {
		g.generateBasic(name)
		return
	}

	g.Printf("\nfor %c := range %s {", g.loopVar, name)
	innerName := fmt.Sprintf("%s[%c]", name, g.loopVar)
	g.loopVar++ // increment loopVar for each nested loop
	g.generate(innerName, t.Elem())
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

func (g *generator) generatePointer(name string, t *types.Pointer) {
	// TODO: eliminate &*
	g.Printf("\nif %v != nil {", name)
	g.generateBasic("ptrTrue")
	g.generate("(*"+name+")", t.Elem())
	g.Printf("} else {")
	g.generateBasic("ptrFalse")
	g.Printf("}\n")
}

func (g *generator) generateString(name string) {
	g.Printf("\nstr = (*reflect.StringHeader)(unsafe.Pointer(&%s))", name)
	g.generateBasic("str.Len")
	g.Printf("if str.Len != 0 {")
	g.Printf(writeTempl, "str.Data", "str.Len")
	g.Printf("}")
}

func (g *generator) generateSlice(name string, t *types.Slice) {
	g.Printf("\nsli = (*reflect.SliceHeader)(unsafe.Pointer(&%s))", name)
	g.generateBasic("sli.Len")

	if isContiguous(t.Elem()) {
		g.Printf(writeTempl, "sli.Data", fmt.Sprintf("sli.Len * int(unsafe.Sizeof(%s[0]))", name))
		return
	}

	g.Printf("\nfor %c := range %s {", g.loopVar, name)
	innerName := fmt.Sprintf("%s[%c]", name, g.loopVar)
	g.loopVar++ // increment loopVar for each nested loop
	g.generate(innerName, t.Elem())
	g.loopVar--
	g.Printf("}\n")
}
