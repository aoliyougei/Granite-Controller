package render

import "testing"

func TestUptime(t *testing.T) {
	for _, tc := range []struct {
		seconds int64
		want    string
	}{{0, "0 秒"}, {59, "59 秒"}, {60, "1 分钟"}, {3660, "1 小时 1 分钟"}, {90060, "1 天 1 小时 1 分钟"}} {
		if got := Uptime(tc.seconds); got != tc.want {
			t.Fatalf("Uptime(%d)=%q", tc.seconds, got)
		}
	}
}
func TestVMStatusText(t *testing.T) {
	if got := VMStatus(3052, "MS-SQL-1", "running", 12540); got != "🟢 VM 3052（MS-SQL-1）当前状态：running，运行时长 3 小时 29 分钟。" {
		t.Fatalf("got=%q", got)
	}
	if got := VMStatus(3052, "", "stopped", 0); got != "🔴 VM 3052 当前状态：stopped。" {
		t.Fatalf("got=%q", got)
	}
	if got := VMStatus(3052, "x", "paused", 0); got != "🟡 VM 3052（x）当前状态：paused。" {
		t.Fatalf("got=%q", got)
	}
}
func TestSubmittedText(t *testing.T) {
	want := map[string]string{"start": "▶️ VM 3052 的启动请求已提交。", "shutdown": "⏻ VM 3052 的正常关机请求已提交。", "stop": "⚠️ VM 3052 的强制停止请求已提交。", "reboot": "🔄 VM 3052 的重启请求已提交。"}
	for action, text := range want {
		if got := Submitted(action, 3052); got != text {
			t.Fatalf("%s=%q", action, got)
		}
	}
}
