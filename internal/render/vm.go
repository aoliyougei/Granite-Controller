package render

import (
	"fmt"
	"strings"
)

func Uptime(seconds int64) string {
	if seconds<=0{return "0 秒"}
	days:=seconds/86400; hours:=seconds%86400/3600; minutes:=seconds%3600/60; secs:=seconds%60
	parts:=[]string{}
	if days>0{parts=append(parts,fmt.Sprintf("%d 天",days))}
	if hours>0{parts=append(parts,fmt.Sprintf("%d 小时",hours))}
	if minutes>0{parts=append(parts,fmt.Sprintf("%d 分钟",minutes))}
	if len(parts)==0{parts=append(parts,fmt.Sprintf("%d 秒",secs))}
	return strings.Join(parts," ")
}
func VMStatus(vmid int64,name,status string,uptime int64)string{
	label:=fmt.Sprintf("VM %d ",vmid); if name!=""{label=fmt.Sprintf("VM %d（%s）",vmid,name)}
	icon:="🟡"; if status=="running"{icon="🟢"}; if status=="stopped"{icon="🔴"}
	if status=="running"{return fmt.Sprintf("%s %s当前状态：%s，运行时长 %s。",icon,label,status,Uptime(uptime))}
	return fmt.Sprintf("%s %s当前状态：%s。",icon,label,status)
}
func Submitted(action string,vmid int64)string{
	return map[string]string{"start":fmt.Sprintf("▶️ VM %d 的启动请求已提交。",vmid),"shutdown":fmt.Sprintf("⏻ VM %d 的正常关机请求已提交。",vmid),"stop":fmt.Sprintf("⚠️ VM %d 的强制停止请求已提交。",vmid),"reboot":fmt.Sprintf("🔄 VM %d 的重启请求已提交。",vmid)}[action]
}
