use pane_core::SafeVm;

#[test]
fn test_vmm_lifecycle() {
    match SafeVm::create() {
        Ok(vm) => {
            println!("VMM created successfully!");
            assert!(vm.get_kvm_fd() >= 0);
            assert!(vm.get_vm_fd() >= 0);
            
            // Try initializing irqchip
            if let Err(e) = vm.init_irqchip() {
                println!("Warning: failed to init irqchip (may not be supported on this host/VM): {:?}", e);
            }

            // Create vCPU 0
            vm.vcpu_create(0).expect("Failed to create vCPU 0");
            let vcpu_fd = vm.get_vcpu_fd(0).expect("Failed to get vCPU fd");
            assert!(vcpu_fd >= 0);

            // Fetch and set regs
            let regs = vm.vcpu_get_regs(0).expect("Failed to get vCPU regs");
            vm.vcpu_set_regs(0, &regs).expect("Failed to set vCPU regs");
        }
        Err(e) => {
            println!("Skipping VMM lifecycle test. Either /dev/kvm is not accessible, or not run on a KVM host. Error: {:?}", e);
        }
    }
}
