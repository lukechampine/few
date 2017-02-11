package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"
)

// BenchmarkCastSmall-4   	2000000000	         1.65 ns/op	       0 B/op	       0 allocs/op
func BenchmarkCastSmall(b *testing.B) {
	b.ReportAllocs()
	var x int
	for i := 0; i < b.N; i++ {
		_ = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{uintptr(unsafe.Pointer(&x)), int(unsafe.Sizeof(x)), int(unsafe.Sizeof(x))}))
	}
}

// BenchmarkCastLarge-4   	2000000000	         1.67 ns/op	       0 B/op	       0 allocs/op
func BenchmarkCastLarge(b *testing.B) {
	b.ReportAllocs()
	var x [1000]int
	for i := 0; i < b.N; i++ {
		_ = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{uintptr(unsafe.Pointer(&x)), int(unsafe.Sizeof(x)), int(unsafe.Sizeof(x))}))
	}
}

// BenchmarkCastPtr-4     	   1000000	         1686 ns/op	       0 B/op	       0 allocs/op
func BenchmarkCastPtr(b *testing.B) {
	b.ReportAllocs()
	var x [1000]*int
	for i := range x {
		x[i] = new(int)
	}
	for i := 0; i < b.N; i++ {
		for j := range x {
			_ = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{uintptr(unsafe.Pointer(x[j])), int(unsafe.Sizeof(x[j])), int(unsafe.Sizeof(x[j]))}))
		}
	}
}

// BenchmarkJSON-4        	    30000	        57213 ns/op	   12840 B/op	       8 allocs/op
func BenchmarkJSON(b *testing.B) {
	b.ReportAllocs()
	var x [1000]*int
	for i := range x {
		x[i] = new(int)
	}
	for i := 0; i < b.N; i++ {
		json.Marshal(x)
	}
}
