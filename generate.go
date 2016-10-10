
import (
	"bytes"
	"fmt"
	"go/types"
	"strings"
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
	for i := 0; i < st.NumFields(); i++ {
		// slow path
		if !isContiguous(st.Field(i).Type()) {
			g.generate(name+"."+st.Field(i).Name(), st.Field(i).Type())
			continue
		}

		// group contiguous fields into a single write
		// NOTE: struct padding will be included
		var sizes []string
		for j := i; j < st.NumFields() && isContiguous(st.Field(j).Type()); j++ {
			sizes = append(sizes, fmt.Sprintf("unsafe.Sizeof(%s.%s)", name, st.Field(j).Name()))
		}
		groupStart := fmt.Sprintf("uintptr(unsafe.Pointer(&%s.%s))", name, st.Field(i).Name())
		groupSize := fmt.Sprintf("int(%s)", strings.Join(sizes, "+"))
		g.Printf(writeTempl, groupStart, groupSize)
		i += len(sizes) - 1
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
