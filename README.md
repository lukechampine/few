# few
#### *Fastest Encoder in the West*
[![Go Report Card](http://goreportcard.com/badge/github.com/lukechampine/few)](https://goreportcard.com/report/github.com/lukechampine/few)

```
go get github.com/lukechampine/few
```

few aims to provide the fastest possible encoder for Go data. It accomplishes this by using `unsafe.Pointer` to cast arbitrary data types to byte slices. This operation is constant-time -- casting a `[100]int` takes no longer than casting a single `int` -- and on most machines it takes under 10 nanoseconds. few can encode a 10MB object at 1 PB/s!

But pointers throw a wrench in the works. If an object contains a pointer (or a string, or a slice), we can no longer simply cast it to a `[]byte` and call it a day; we have to encode all the pointed-to data as well. That slows things down quite a bit. So you'll get the most bang for your buck by encoding large objects without many pointers. And of course, the usual "cast-encoding" caveats apply here: few is not portable across architectures or languages, and it sacrifices some compactness, especially when the object is sparse.

While few encodes pointed-to data, it doesn't encode the pointers themselves. Unfortunately, this means we can't cast an entire struct/array/slice at once if it contains pointers. I think this is an acceptable trade-off, since following pointers incurs overhead anyway. Besides, few is practically useless already; at least being deterministic is *slightly* useful! few is also injective (for a given type): if two objects contain different data, they are guaranteed to have different encodings (with one exception -- see specification). These properties taken together mean that you easily create content hashes by piping few-encoded data to your favorite hashing function. Just be aware that the hash, like the data itself, is not portable.

few is not a library, it is a code-generation tool. Unlike `encoding/json` and friends, there is no generic reflection-based `Marshal` function that you call at runtime. Reflection is slow. If you're going to eat the cost of reflection, you might as well use a portable encoding format while you're at it. On that note, the non-portable nature of few allows us to omit schema files: the schema is just the Go type declaration. Simply run `few` and it will generate a `WriteTo` method for the specified types.

## Specification

In general, objects are encoded as their underlying bytes according to Go's memory model. For example:
```go
true       -> []byte{1}
int8(12)   -> []byte{12}
uint64(12) -> []byte{0, 0, 0, 0, 0, 0, 0, 12}
```
Strings are encoded as though they were cast to bytes:
```go
"foo" -> []byte{102, 111, 111}
```
Pointers are encoded as the encoding of their dereferenced value. `nil` pointers are encoded as `uintptr(0)`:
```go
***int8(12) -> []byte{12}
(*int)(nil) -> []byte{0, 0, 0, 0, 0, 0, 0, 0}
```
Slices, arrays, and structs are encoded as the concatenation of their encoded elements:
```go
[3]int16{1, 2, 3}         -> []byte{0, 1, 0, 2, 0, 3}
[][]bool{{true}, {false}} -> []byte{1, 0}
struct{A, B int8}{4, 5}   -> []byte{4, 5}
```
Importantly, the capacity of a slice is not present in the encoding. This means that two slices with the same data (and length) but different capacity will not be distinguishable. In practice, I do not expect this to be a problem.

Maps, channels, and function pointers are not supported.