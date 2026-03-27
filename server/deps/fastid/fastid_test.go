package fastid_test

import (
	"fmt"
	"game/deps/fastid"
	"testing"
	"time"
)

func TestConfig_GetTimeMillFromFastID(t *testing.T) {

	fastid.InitWithMachineID(111)
	id := fastid.GenInt64ID()
	x := time.Now().UnixMilli()
	got := fastid.CommonConfig.GetTimeMillFromFastID(id)
	y := fastid.CommonConfig.GetTimeFromID(id)
	fmt.Println(got, x, y)
	// TODO: update the condition below to compare got with tt.want.
}

func BenchmarkGetTimeMillFromFastID(b *testing.B) {
	fastid.InitWithMachineID(111)
	id := fastid.GenInt64ID()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id += 1
			_ = fastid.GetTimeMillFromFastID(id)
		}
	})
}
