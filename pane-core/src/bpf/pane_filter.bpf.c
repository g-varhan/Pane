#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);      // IPv4 address (network byte order)
    __type(value, __u32);    // Group ID
} ip_groups SEC(".maps");

SEC("classifier")
int pane_filter(struct __sk_buff *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) {
        return TC_ACT_OK;
    }

    // Only process IPv4 packets. Allow ARP and other protocols.
    if (bpf_ntohs(eth->h_proto) != ETH_P_IP) {
        return TC_ACT_OK;
    }

    struct iphdr *ip = (struct iphdr *)(eth + 1);
    if ((void *)(ip + 1) > data_end) {
        return TC_ACT_OK;
    }

    __u32 src_ip = ip->saddr;
    __u32 dest_ip = ip->daddr;

    __u32 *src_group = bpf_map_lookup_elem(&ip_groups, &src_ip);
    __u32 *dest_group = bpf_map_lookup_elem(&ip_groups, &dest_ip);

    // If both source and destination IPs are tracked in group rules,
    // enforce micro-segmentation.
    if (src_group && dest_group) {
        if (*src_group != *dest_group) {
            return TC_ACT_SHOT; // Drop packet
        }
    }

    return TC_ACT_OK; // Pass packet
}

char _license[] SEC("license") = "GPL";
