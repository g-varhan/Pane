// SPDX-License-Identifier: Apache-2.0

fn main() {
    let project_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
    println!("cargo:rustc-link-search=native={}/../pane-vmm", project_dir);
    println!("cargo:rustc-link-lib=static=pane_vmm");
    println!("cargo:rustc-link-lib=dylib=uring");
    println!("cargo:rerun-if-changed=../pane-vmm/libpane_vmm.a");
    println!("cargo:rerun-if-changed=../pane-vmm/include/pane_vmm.h");
    println!("cargo:rerun-if-changed=src/bpf/pane_filter.bpf.c");

    // Compile eBPF program using clang
    let out_dir = std::env::var("OUT_DIR").unwrap();
    let bpf_o_path = std::path::Path::new(&out_dir).join("pane_filter.bpf.o");

    let status = std::process::Command::new("clang")
        .args([
            "-O2",
            "-target",
            "bpf",
            "-g",
            "-c",
            "src/bpf/pane_filter.bpf.c",
            "-o",
            bpf_o_path.to_str().unwrap(),
        ])
        .status()
        .expect("Failed to run clang for eBPF compilation");

    if !status.success() {
        panic!("Failed to compile eBPF program pane_filter.bpf.c");
    }

    println!(
        "cargo:rustc-env=BPF_OBJ_PATH={}",
        bpf_o_path.to_str().unwrap()
    );
}
