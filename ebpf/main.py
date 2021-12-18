# -*- coding: utf-8 -*-
from __future__ import print_function
from bcc import BPF

bpf_text="""
#include <uapi/linux/ptrace.h>

struct data_t {
    u64 data;
};

BPF_PERF_OUTPUT(trace);

int getRandomNumberCalled(struct pt_regs *ctx) {
  struct data_t now_data;
  u64 arg_address = ctx->sp;
  
  arg_address = ctx->sp+8;
  bpf_probe_read(&now_data.data, 8, (void*)arg_address);
  bpf_trace_printk("RET %d\\n", now_data.data);
  //bpf_probe_write_user((void*)ctx->sp+16, &now_data.data,8);
  
  arg_address = ctx->sp+0x40;
  bpf_probe_read(&now_data.data, 8, (void*)arg_address);
  bpf_trace_printk("RET %d\\n", now_data.data);
  
  trace.perf_submit(ctx, &now_data, sizeof(now_data));
  bpf_trace_printk("RET ==========\\n");
  return 0;
}
"""


b = BPF(text=bpf_text)
# b.attach_uprobe(
#     name="/media/psf/Home/go/src/ebpf-share/monitor-func/main",
#     sym="main.getRandomNumber",
#     fn_name="getRandomNumberCalled",
# )
b.attach_uretprobe(
    name="/media/psf/Home/go/src/ebpf-share/monitor-func/main",
    sym="main.getRandomNumber",
    fn_name="getRandomNumberCalled",
)


def print_event(cpu, data, size):
    out = b["trace"].event(data)
    print("random number:", out.data)


# loop with callback to print_event
b["trace"].open_perf_buffer(print_event)

while 1:
    b.kprobe_poll()
