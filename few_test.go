package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"
)

// BenchmarkCastSmall-4   	2000000000	         1.59 ns/op
func BenchmarkCastSmall(b *testing.B) {
	var x int
	for i := 0; i < b.N; i++ {
		_ = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{uintptr(unsafe.Pointer(&x)), int(unsafe.Sizeof(x)), int(unsafe.Sizeof(x))}))
	}
}

// BenchmarkCastLarge-4   	2000000000	         1.60 ns/op
func BenchmarkCastLarge(b *testing.B) {
	var x [1000]int
	for i := 0; i < b.N; i++ {
		_ = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{uintptr(unsafe.Pointer(&x)), int(unsafe.Sizeof(x)), int(unsafe.Sizeof(x))}))
	}
}

// BenchmarkCastPtr-4     	 1000000	      1602 ns/op
func BenchmarkCastPtr(b *testing.B) {
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

// BenchmarkCastPtr-4     	 1000000	      1602 ns/op
func BenchmarkJSON(b *testing.B) {
	var x [1000]*int
	for i := range x {
		x[i] = new(int)
	}
	for i := 0; i < b.N; i++ {
		json.Marshal(x)
	}
}
