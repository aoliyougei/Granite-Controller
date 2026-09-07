package logic

import "testing"

func TestNormalizeChineseVMStartAcceptsSupportedPhrases(t *testing.T) {
	tests := []string{
		"开启 3052 这个 VM",
		"启动虚拟机 3052",
		"把 VM 3052 开起来",
		"给 3052 号虚拟机开机",
		"打开 VM 3052",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := NormalizeChineseVMStart(input)
			if err != nil {
				t.Fatalf("NormalizeChineseVMStart() error = %v", err)
			}
			if got.VMID != 3052 || got.NeedleMessage != "Start VM 3052" {
				t.Fatalf("normalized = %+v", got)
			}
		})
	}
}

func TestNormalizeChineseVMStartRejectsUnsafeOrUnclearPhrases(t *testing.T) {
	tests := []string{
		"不要开启 VM 3052",
		"别启动虚拟机 3052",
		"禁止打开 VM 3052",
		"无需给 VM 3052 开机",
		"取消启动 VM 3052",
		"启动 VM 3052 和 3053",
		"启动服务器 3052",
		"关闭 VM 3052",
		"重启 VM 3052",
		"重新启动 VM 3052",
		"开启 VMware 3052",
		"开启 VM",
		"开启 VM 0",
		"开启 VM 9223372036854775808",
		"start VM 3052",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := NormalizeChineseVMStart(input)
			assertLogicCode(t, err, "MESSAGE_INVALID")
		})
	}
}
