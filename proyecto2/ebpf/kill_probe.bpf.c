#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct kill_event {
    u32 pid_caller;
    u32 pid_target;
    s32 sig;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16MB ring buffer
} events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_kill")
int trace_sys_enter_kill(struct trace_event_raw_sys_enter *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid_caller = pid_tgid >> 32;

    s32 pid_target = (s32)ctx->args[0];
    s32 sig = (s32)ctx->args[1];

    if (sig == 9 || sig == 15) {
        struct kill_event *event = bpf_ringbuf_reserve(&events, sizeof(struct kill_event), 0);
        if (event) {
            event->pid_caller = pid_caller;
            event->pid_target = (u32)pid_target;
            event->sig = sig;
            bpf_ringbuf_submit(event, 0);
        }
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
