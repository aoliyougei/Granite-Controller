package managed

import "testing"

func TestNormalizeVMStartAcceptsSupportedChinesePhrases(t *testing.T) {
	for _, value := range []string{"开启 3052 这个 VM", "启动虚拟机 3052", "把 VM 3052 开起来", "给 3052 号虚拟机开机", "打开 VM 3052"} {
		t.Run(value, func(t *testing.T) {
			got, err := NormalizeVMStart(Input{Original: value})
			if err != nil {
				t.Fatal(err)
			}
			if got.VMID != 3052 || got.Normalized != "Start VM 3052" || got.Original != value {
				t.Fatalf("command=%+v", got)
			}
		})
	}
}
func TestNormalizeVMStartRejectsUnsafeOrUnsupportedPhrases(t *testing.T) {
	for _, value := range []string{"不要开启 VM 3052", "别启动虚拟机 3052", "禁止打开 VM 3052", "无需开启 VM 3052", "取消启动 VM 3052", "关闭 VM 3052", "关机 VM 3052", "停止 VM 3052", "重启 VM 3052", "重新启动 VM 3052", "启动 VM 3052 和 3053", "开启 VMware 3052", "开启 VM", "开启 VM 0", "开启 VM 9223372036854775808", "Start VM 3052", "开启服务器 3052"} {
		t.Run(value, func(t *testing.T) {
			_, err := NormalizeVMStart(Input{Original: value})
			if err == nil || err.Code != "unsupported_managed_command" {
				t.Fatalf("error=%+v", err)
			}
		})
	}
}
