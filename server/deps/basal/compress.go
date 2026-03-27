package basal

import (
	"bytes"
	"compress/gzip"
	"github.com/golang/snappy"
	"github.com/pierrec/lz4/v4"
	"io"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, err := gzip.NewWriterLevel(nil, 1)
		if err != nil {
			panic(err)
		}
		return w
	}}

func GZipCompress(data []byte) []byte {
	w := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(w)
	var in bytes.Buffer
	w.Reset(&in)
	w.Write(data)
	w.Close()
	return in.Bytes()

	//var in bytes.Buffer
	//defer in.Reset()
	//w, err := gzip.NewWriterLevel(&in, 1)
	//if err != nil {
	//	panic(err)
	//}
	//w.Write(data)
	//w.Close()
	//return in.Bytes()
}

func GZipDecompress(data []byte) ([]byte, error) {
	b := bytes.NewReader(data)
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// 压缩快 解压慢
func SnappyCompress(data []byte) []byte {
	return snappy.Encode(nil, data)
}

func SnappyDecompress(data []byte) ([]byte, error) {
	return snappy.Decode(nil, data)
}

// 压缩一般 解压快
func LZ4Compress(data []byte) ([]byte, error) {
	buf := make([]byte, lz4.CompressBlockBound(len(data)))
	if n, err := lz4.CompressBlock(data, buf, nil); err != nil {
		return nil, err
	} else {
		return buf[:n], nil
	}
}

func LZ4Decompress(data []byte) ([]byte, error) {
	buf := make([]byte, 3*len(data))
	if n, err := lz4.UncompressBlock(data, buf); err != nil {
		return nil, err
	} else {
		return buf[:n], nil
	}
}
