# few
#### *Fastest Encoder in the West*
[![Go Report Card](http://goreportcard.com/badge/github.com/lukechampine/few)](https://goreportcard.com/report/github.com/lukechampine/few)

```
go get github.com/lukechampine/few
```

few aims to provide the fastest possible encoder for Go data. It accomplishes
this by using `unsafe.Pointer` to cast arbitrary data types to byte slices.
This operation is constant-time -- casting a `[100]int` takes no longer than
casting a single `int` -- and on most machines it takes under 10 nanoseconds.
few can encode a 10MB object at 1 PB/s!

But pointers throw a wrench in the works. If an object contains a pointer (or
a string, or a slice), we can no longer simply cast it to a `[]byte` and call
it a day; we have to encode all the pointed-to data as well. That slows things
down quite a bit. So you'll get the most bang for your buck by encoding large
objects without many pointers. And of course, the usual "cast-encoding"
caveats apply here: few is not portable across architectures or languages, and
it sacrifices some compactness, especially when the object is sparse.

few destroys pointer information (valid pointers are converted to `true`, nil
pointers to `false`), but it is otherwise injective; that is, if two objects
contain different data, they are guaranteed to have different encodings. This
means that you can easily create content hashes by piping few-encoded data to
your favorite hashing function. Just be aware that the hash, like the data
itself, will not be portable.

few is not a library, it is a code-generation tool. Unlike `encoding/json` and
friends, there is no generic reflection-based `Marshal` function that you call
at runtime. Reflection is slow. If you're going to eat the cost of reflection,
you might as well use a portable encoding format while you're at it. On that
note, the non-portable nature of few allows us to omit schema files: the
schema is the Go type declaration. Just run `few` and it will generate a
`WriteTo` method for the specified types. (If you want a `[]byte`, write to a
`bytes.Buffer`.)

<sup><sub>also there's no decoder (yet) but hey if you're just hashing data who cares</sub></sup>

## Benchmarks

As far as I can tell, few outperforms all of the encoders in the [Go Serialization Benchmarks](https://github.com/alecthomas/go_serialization_benchmarks)
test suite in both speed and allocations, but that's not saying much since
their test framework doesn't reflect how the encoders (especially the fast
ones) will be used in actual code.

On my machine, few encodes the following object (67 bytes) in 31 ns with no
allocations:

```go
var A struct {
	Name     string
	BirthDay int64
	Phone    string
	Siblings int
	Spouse   bool
	Money    float64
}{
	Name:     "Foo Bar",
	BirthDay: 233431200,
	Phone:    "123-456-7890",
	Siblings: 12,
	Spouse:   true,
	Money:    1e9,
}
```

## Specification

Basic objects are encoded as their underlying bytes according to Go's memory
model. For example:

```go
true       -> []byte{1}
int8(12)   -> []byte{12}
uint64(12) -> []byte{12, 0, 0, 0, 0, 0, 0, 0}
```

Pointers are encoded as the encoding of their dereferenced value, prefixed by
`byte(1)` (i.e. boolean true). `nil` pointers are encoded as a `0` byte.

```go
***int8(12) -> []byte{1, 1, 1, 12}
(*int)(nil) -> []byte{0}
```

Arrays and structs are encoded as the concatenation of their encoded elements:

```go
[3]int16{1, 2, 3}       -> []byte{1, 0, 2, 0, 3, 0}
struct{A, B int8}{4, 5} -> []byte{4, 5}
```

If adjacent struct fields are "contiguous" (they do not contain pointers), an
optimization is applied: all of the fields are encoded with a single cast. As
a consequence, the encoding may include struct padding. Non-contiguous fields
(and single contiguous fields) are encoded using the standard recursive
approach, so they will not contain padding.

Slices and strings are encoded as the concatenation of their encoded elements,
but prefixed with an `int` specifying their length:

```go
"foo"               -> []byte{3, 0, 0, 0, 0, 0, 0, 0, 102, 111, 111}
[]bool{true, false} -> []byte{2, 0, 0, 0, 0, 0, 0, 0, 1, 0}
```

Importantly, the capacity of a slice is not present in the encoding. This
means that two slices with the same data (and length) but different capacity
will not be distinguishable. In practice, I do not expect this to be a
problem, but it's easy to add later.


Maps, channels, and function pointers are not supported.