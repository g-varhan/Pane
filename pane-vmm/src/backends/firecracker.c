// SPDX-License-Identifier: Apache-2.0

#include "pane_vmm.h"
#include <errno.h>
#include <linux/kvm.h>
#include <stdio.h>
#include <string.h>
#include <sys/ioctl.h>

extern void *pane_vm_gpa_to_hva(pane_vm_t *vm, uint64_t gpa);

int pane_vm_setup_firecracker_mode(pane_vm_t *vm, uint32_t vcpu_id,
                                   uint64_t entry_point) {
  if (!vm)
    return -EINVAL;

  // 1. Set up GDT in guest memory at 0x1000
  void *gdt_hva = pane_vm_gpa_to_hva(vm, 0x1000);
  if (!gdt_hva) {
    return -EFAULT; // Guest memory not mapped at 0x1000
  }
  uint64_t *gdt = (uint64_t *)gdt_hva;
  gdt[0] = 0; // Null descriptor
  gdt[1] =
      0x00af9b000000ffffULL; // CS: 64-bit, present, code exec/read, ring 0, L=1
  gdt[2] = 0x00cf93000000ffffULL; // DS/SS/ES: 64-bit, present, data read/write,
                                  // ring 0

  // 2. Set up Page Tables in guest memory
  // PML4 at 0x2000, PDPT at 0x3000, PD at 0x4000
  void *pml4_hva = pane_vm_gpa_to_hva(vm, 0x2000);
  void *pdpt_hva = pane_vm_gpa_to_hva(vm, 0x3000);
  void *pd_hva = pane_vm_gpa_to_hva(vm, 0x4000);
  if (!pml4_hva || !pdpt_hva || !pd_hva) {
    return -EFAULT;
  }

  memset(pml4_hva, 0, 4096);
  memset(pdpt_hva, 0, 4096);
  memset(pd_hva, 0, 4096);

  uint64_t *pml4 = (uint64_t *)pml4_hva;
  uint64_t *pdpt = (uint64_t *)pdpt_hva;
  uint64_t *pd = (uint64_t *)pd_hva;

  // Link PML4[0] -> PDPT
  pml4[0] = 0x3000 | 0x3; // Present + Writable

  // Link PDPT[0] -> PD
  pdpt[0] = 0x4000 | 0x3; // Present + Writable

  // Map first 1GB identity-mapped using 2MB huge pages
  for (int i = 0; i < 512; i++) {
    pd[i] =
        (i * 2 * 1024 * 1024ULL) | 0x83; // Present + Writable + PageSize (2MB)
  }

  // 3. Configure vCPU special registers (sregs) via KVM_SET_SREGS
  int vcpu_fd = pane_vm_get_vcpu_fd(vm, vcpu_id);
  if (vcpu_fd < 0) {
    return -EINVAL;
  }

  struct kvm_sregs sregs;
  if (ioctl(vcpu_fd, KVM_GET_SREGS, &sregs) == -1) {
    return -errno;
  }

  // Set GDT descriptor
  sregs.gdt.base = 0x1000;
  sregs.gdt.limit = 23; // 3 entries of 8 bytes - 1

  // CS descriptor config
  sregs.cs.selector = 0x8;
  sregs.cs.base = 0;
  sregs.cs.limit = 0xffffffff;
  sregs.cs.type = 11; // Code execute/read
  sregs.cs.s = 1;     // Non-system descriptor
  sregs.cs.dpl = 0;
  sregs.cs.present = 1;
  sregs.cs.db = 0; // Must be 0 for 64-bit long mode CS
  sregs.cs.l = 1;  // 1 for 64-bit CS
  sregs.cs.g = 1;  // Page granularity

  // Data segments: DS, ES, FS, GS, SS
  struct kvm_segment data_seg = {
      .selector = 0x10,
      .base = 0,
      .limit = 0xffffffff,
      .type = 3, // Read/write data
      .s = 1,
      .dpl = 0,
      .present = 1,
      .db = 1, // default 32-bit limit
      .l = 0,
      .g = 1,
  };

  sregs.ds = data_seg;
  sregs.es = data_seg;
  sregs.fs = data_seg;
  sregs.gs = data_seg;
  sregs.ss = data_seg;

  // Load CR3 with PML4 base
  sregs.cr3 = 0x2000;

  // Enable Paging (PG = 0x80000000) and Protected Mode (PE = 0x1)
  sregs.cr0 |= 0x80000011;

  // Enable Physical Address Extension (PAE = 0x20)
  sregs.cr4 |= 0x20;

  // Enable Long Mode Enable (LME = 0x100) and Long Mode Active (LMA = 0x400)
  sregs.efer |= 0x500;

  if (ioctl(vcpu_fd, KVM_SET_SREGS, &sregs) == -1) {
    return -errno;
  }

  // 4. Configure general purpose registers (RIP, RSP, RFLAGS)
  struct kvm_regs regs = {
      .rip = entry_point,
      .rflags = 2,
  };
  // Let's set guest stack to 0x200000 (2MB), growing downwards
  regs.rsp = 0x200000;

  if (ioctl(vcpu_fd, KVM_SET_REGS, &regs) == -1) {
    return -errno;
  }

  return 0;
}
