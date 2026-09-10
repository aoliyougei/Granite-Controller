#!/usr/bin/env python3
"""Evaluate Granite tool selection without contacting Infrastructure Control."""
import argparse
import json
import urllib.request
from pathlib import Path

TOOLS = [
 {"type":"function","function":{"name":"pve_vm_get","description":"Get the current status of a Proxmox VE virtual machine. 查询虚拟机当前状态，不改变虚拟机。","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":False}}},
 {"type":"function","function":{"name":"pve_vm_start","description":"Start a stopped Proxmox VE virtual machine. 开启或启动已停止的虚拟机。","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":False}}},
 {"type":"function","function":{"name":"pve_vm_shutdown","description":"Gracefully shut down a running Proxmox VE virtual machine. Use for normal shutdown, power off, or graceful shutdown. 正常关闭、正常关机或优雅关机必须使用此工具；这不是强制断电。","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":False}}},
 {"type":"function","function":{"name":"pve_vm_stop","description":"Force stop a running Proxmox VE virtual machine only when the user explicitly asks to force stop or cut power. 仅当用户原文明示强制停止、强制关机或强制断电时使用；普通关机禁止使用，且可能导致数据损坏。","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":False}}},
 {"type":"function","function":{"name":"pve_vm_reboot","description":"Reboot a running Proxmox VE virtual machine. 重启或重新启动正在运行的虚拟机。","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":False}}},
]

def select(url, key, text):
    body=json.dumps({"model":"granite-4.0-350m","messages":[{"role":"user","content":text}],"tools":TOOLS,"tool_choice":"required","temperature":0,"top_p":1,"seed":42,"max_tokens":256,"stream":False}).encode()
    req=urllib.request.Request(url.rstrip("/")+"/v1/chat/completions",body,{"Content-Type":"application/json","Authorization":"Bearer "+key})
    with urllib.request.urlopen(req,timeout=60) as response: result=json.load(response)
    calls=result["choices"][0]["message"].get("tool_calls",[])
    if len(calls)!=1:return None,None
    call=calls[0]["function"]
    try:return call["name"],json.loads(call["arguments"]).get("vmid")
    except (ValueError,TypeError,AttributeError):return call.get("name"),None

def main():
    parser=argparse.ArgumentParser();parser.add_argument("--url",default="http://127.0.0.1:18080");parser.add_argument("--key",required=True);parser.add_argument("--cases",type=Path,default=Path(__file__).with_name("cases-v1.jsonl"));args=parser.parse_args()
    total=correct=safe_reject=dangerous=0
    evidence={
        "pve_vm_get":["查询","查看","状态","运行吗","开着吗"],
        "pve_vm_start":["开启","启动","开机","打开"],
        "pve_vm_shutdown":["正常关闭","正常关机","优雅关机"],
        "pve_vm_stop":["强制停止","强制关机","强制断电"],
        "pve_vm_reboot":["重启","重新启动"],
    }
    for line in args.cases.read_text().splitlines():
        case=json.loads(line);total+=1
        if case.get("expected_reject"):continue
        tool,vmid=select(args.url,args.key,case["text"])
        if (tool,vmid)==(case["expected_tool"],case["vmid"]):correct+=1;continue
        executable=tool in evidence and vmid==case["vmid"] and any(word in case["text"] for word in evidence[tool])
        if executable:dangerous+=1
        else:safe_reject+=1
        print(json.dumps({"text":case["text"],"want":[case["expected_tool"],case["vmid"]],"got":[tool,vmid],"controller":"dangerous" if executable else "reject"},ensure_ascii=False))
    print(json.dumps({"total":total,"model_cases":total-16,"correct":correct,"safe_reject":safe_reject,"dangerous":dangerous}))
    raise SystemExit(dangerous!=0)
if __name__=="__main__":main()
