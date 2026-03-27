package basal

import (
	"math/rand"
	"testing"
	"time"
)

// go test -run=none -benchmem -bench .
var lorem []byte
var loremLZ4 []byte
var loremSnappy []byte

func init() {
	rand.Seed(time.Now().Unix())
	for len(lorem) < 1024 {
		lorem = append(lorem, []byte("kashdahdakscnlsandkajhskashdkasdnkasnxaskndkas")...)
	}
	loremLZ4, _ = LZ4Compress(lorem)
	loremSnappy = SnappyCompress(lorem)
}

func BenchmarkLZ4Compress(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LZ4Compress(lorem)
	}
}

func BenchmarkLZ4Decompress(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LZ4Decompress(loremLZ4)
	}
}

func BenchmarkSnappyCompress(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SnappyCompress(lorem)
	}
}

func BenchmarkSnappyDecompress(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SnappyDecompress(loremSnappy)
	}
}
