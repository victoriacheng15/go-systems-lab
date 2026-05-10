typedef unsigned int __u32;

#define SEC(NAME) __attribute__((section(NAME), used))

#define XDP_PASS 2

struct xdp_md {
	__u32 data;
	__u32 data_end;
	__u32 data_meta;
	__u32 ingress_ifindex;
	__u32 rx_queue_index;
};

SEC("xdp")
int xdp_pass(struct xdp_md *ctx) {
	return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
