package options

import (
	"game/src/configdoc"
	"testing"
)

func TestSetNetCfgSetsTimeout(t *testing.T) {
	opt := NewMsgQueOptions()
	cfg := &configdoc.Net{
		CltReadTimeout:     300,
		CltReadBufferSize:  4096,
		CltWriteBufferSize: 8192,
		CltWriteChanSize:   128,
		CltEnableDH:        true,
		Compress:           true,
		CompressMode:       1,
		CompressLimit:      1024,
		DelayWrite:         5,
	}

	opt.SetNetCfg(cfg)

	if opt.Timeout != cfg.CltReadTimeout {
		t.Fatalf("timeout not applied, got=%d want=%d", opt.Timeout, cfg.CltReadTimeout)
	}
}

func TestSetIsGate(t *testing.T) {
	opt := NewMsgQueOptions()

	opt.SetIsGate(true)

	if !opt.IsGate {
		t.Fatalf("isGate not applied")
	}
}

func TestSetNetCfgNilNoChange(t *testing.T) {
	opt := NewMsgQueOptions()
	opt.Timeout = 123

	opt.SetNetCfg(nil)

	if opt.Timeout != 123 {
		t.Fatalf("timeout should remain unchanged, got=%d want=%d", opt.Timeout, 123)
	}
}
