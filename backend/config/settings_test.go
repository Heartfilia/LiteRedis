package config

import "testing"

func TestEffectiveLocalAPIPort(t *testing.T) {
	original := IsDevBuild()
	defer SetDevBuild(original)

	t.Run("正式版固定 64201", func(t *testing.T) {
		SetDevBuild(false)
		if got := DefaultSettings().LocalAPIPort; got != LocalAPIPortProd {
			t.Fatalf("default = %d", got)
		}
		if got := EffectiveLocalAPIPort(0); got != LocalAPIPortProd {
			t.Fatalf("unset = %d", got)
		}
		// 配置文件被开发版写成 64202 时，正式版要让回 64201
		if got := EffectiveLocalAPIPort(LocalAPIPortDev); got != LocalAPIPortProd {
			t.Fatalf("dev port leaked into prod: %d", got)
		}
	})

	t.Run("开发版固定 64202", func(t *testing.T) {
		SetDevBuild(true)
		if got := DefaultSettings().LocalAPIPort; got != LocalAPIPortDev {
			t.Fatalf("default = %d", got)
		}
		if got := EffectiveLocalAPIPort(0); got != LocalAPIPortDev {
			t.Fatalf("unset = %d", got)
		}
		// 老配置里存着 64201 时开发版要让开
		if got := EffectiveLocalAPIPort(LocalAPIPortProd); got != LocalAPIPortDev {
			t.Fatalf("prod port kept in dev: %d", got)
		}
	})

	t.Run("自定义端口不受影响", func(t *testing.T) {
		SetDevBuild(true)
		if got := EffectiveLocalAPIPort(64300); got != 64300 {
			t.Fatalf("custom port in dev = %d", got)
		}
		SetDevBuild(false)
		if got := EffectiveLocalAPIPort(64300); got != 64300 {
			t.Fatalf("custom port in prod = %d", got)
		}
	})
}
