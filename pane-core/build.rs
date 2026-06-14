fn main() {
    let project_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
    println!("cargo:rustc-link-search=native={}/../pane-vmm", project_dir);
    println!("cargo:rustc-link-lib=static=pane_vmm");
    println!("cargo:rustc-link-lib=dylib=uring");
    println!("cargo:rerun-if-changed=../pane-vmm/libpane_vmm.a");
    println!("cargo:rerun-if-changed=../pane-vmm/include/pane_vmm.h");
}
