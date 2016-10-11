package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/tools/go/loader"
)

func usage() {
	fmt.Fprintln(os.Stderr, `Usage of few:
	few [flags] -type T [package]
	few [flags] -type T files... # Must be a single package
Flags:`)
	flag.PrintDefaults()
}

// pkgSrcDir returns the directory in which pkgPath resides.
func pkgSrcDir(pkgPath string) string {
	// relative path
	if pkgPath == "." || pkgPath == ".." || strings.HasPrefix(pkgPath, "./") || strings.HasPrefix(pkgPath, "../") {
		return pkgPath
	}

	// check stdlib
	stdPath := filepath.Join(runtime.GOROOT(), "src", pkgPath)
	if _, err := os.Stat(stdPath); err == nil {
		return stdPath
	}

	// check gopath
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Fatal("couldn't locate source directory: GOPATH not set")
	}
	gopathPath := filepath.Join(gopath, "src", pkgPath)
	if _, err := os.Stat(gopathPath); err == nil {
		return gopathPath
	}

	// should never happen, since pkgSrcDir is called after we've successfully
	// loaded the package
	panic("couldn't locate source directory for " + pkgPath)
}

func main() {
	log.SetFlags(0)
	typeNames := flag.String("type", "", "comma-separated list of type names; must be set")
	output := flag.String("output", "", "output file name; default srcdir/<type>_few.go")
	flag.Usage = usage
	flag.Parse()
	if len(*typeNames) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	types := strings.Split(*typeNames, ",")

	// load the package
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}
	var conf loader.Config
	_, err := conf.FromArgs(args, false)
	if err != nil {
		log.Fatal(err)
	}
	lprog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}
	pkgs := lprog.InitialPackages()
	if len(pkgs) != 1 {
		log.Fatalf("expected 1 package, got %v", len(pkgs))
	}
	pkg := pkgs[0].Pkg

	// initialize generator and write file header
	var g generator
	g.generateHeader(pkg.Name())

	// generate a WriteTo method for each type
	for _, t := range types {
		obj := pkg.Scope().Lookup(t)
		if obj == nil {
			log.Fatalf("%s.%s not found", pkg.Name(), t)
		}
		g.generateMethod(obj.Name(), obj.Type())
	}

	// format the output
	src := g.format()

	// write to file
	if *output == "" {
		if len(lprog.Created) == 1 {
			// if creating from a set of go files, always output to cwd
			*output = fmt.Sprintf("%s_few.go", filepath.Base(pkg.Path()))
		} else {
			*output = filepath.Join(pkgSrcDir(pkg.Path()), fmt.Sprintf("%s_few.go", filepath.Base(pkg.Path())))
		}
	}
	err = ioutil.WriteFile(*output, src, 0644)
	if err != nil {
		log.Fatalln("couldn't write generated code:", err)
	}
}
