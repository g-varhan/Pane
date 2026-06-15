// SPDX-License-Identifier: Apache-2.0

#include "pane_vmm.h"
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>

int main() {
  struct timespec start, end;
  clock_gettime(CLOCK_MONOTONIC, &start);

  pane_vm_t *vm = NULL;
  int ret = pane_vm_create(&vm);
  if (ret != 0) {
    fprintf(stderr, "Failed to create VM: %s\n", strerror(-ret));
    return 1;
  }

  uint64_t ram_size = 8ULL * 1024 * 1024; // 8 MB guest RAM
  void *ram = mmap(NULL, ram_size, PROT_READ | PROT_WRITE,
                   MAP_ANONYMOUS | MAP_PRIVATE, -1, 0);
  if (ram == MAP_FAILED) {
    perror("mmap RAM");
    pane_vm_destroy(vm);
    return 1;
  }

  ret = pane_vm_set_user_memory_region(vm, 0, 0, ram_size, (uint64_t)ram, 0);
  if (ret != 0) {
    fprintf(stderr, "Failed to set RAM region: %s\n", strerror(-ret));
    munmap(ram, ram_size);
    pane_vm_destroy(vm);
    return 1;
  }

  // Skip legacy PIT/PIC irqchip initialization for MicroVM/Firecracker mode
  // (Strips PIT, PIC, and PIT speaker dummy logic to minimize startup time)
  /*
  ret = pane_vm_init_irqchip(vm);
  if (ret != 0) {
      fprintf(stderr, "Failed to initialize IRQ chip: %s\n", strerror(-ret));
      munmap(ram, ram_size);
      pane_vm_destroy(vm);
      return 1;
  }
  */

  // Set up Virtio-MMIO device at 0x10000000 (256 MB)
  ret = pane_vm_setup_virtio_mmio(vm, 0x10000000, 512, 5);
  if (ret != 0) {
    fprintf(stderr, "Failed to set up Virtio-MMIO: %s\n", strerror(-ret));
    munmap(ram, ram_size);
    pane_vm_destroy(vm);
    return 1;
  }

  // 64-bit long mode payload
  uint8_t code[] = {
      // Write "Hello from 64-bit Long Mode! "
      0xba, 0xf8, 0x03, 0x00, 0x00, // mov edx, 0x3f8
      0xb0, 0x48,                   // mov al, 'H'
      0xee,                         // out dx, al
      0xb0, 0x65,                   // mov al, 'e'
      0xee,                         // out dx, al
      0xb0, 0x6c,                   // mov al, 'l'
      0xee,                         // out dx, al
      0xb0, 0x6c,                   // mov al, 'l'
      0xee,                         // out dx, al
      0xb0, 0x6f,                   // mov al, 'o'
      0xee,                         // out dx, al
      0xb0, 0x20,                   // mov al, ' '
      0xee,                         // out dx, al
      0xb0, 0x66,                   // mov al, 'f'
      0xee,                         // out dx, al
      0xb0, 0x72,                   // mov al, 'r'
      0xee,                         // out dx, al
      0xb0, 0x6f,                   // mov al, 'o'
      0xee,                         // out dx, al
      0xb0, 0x6d,                   // mov al, 'm'
      0xee,                         // out dx, al
      0xb0, 0x20,                   // mov al, ' '
      0xee,                         // out dx, al
      0xb0, 0x36,                   // mov al, '6'
      0xee,                         // out dx, al
      0xb0, 0x34,                   // mov al, '4'
      0xee,                         // out dx, al
      0xb0, 0x2d,                   // mov al, '-'
      0xee,                         // out dx, al
      0xb0, 0x62,                   // mov al, 'b'
      0xee,                         // out dx, al
      0xb0, 0x69,                   // mov al, 'i'
      0xee,                         // out dx, al
      0xb0, 0x74,                   // mov al, 't'
      0xee,                         // out dx, al
      0xb0, 0x20,                   // mov al, ' '
      0xee,                         // out dx, al
      0xb0, 0x4c,                   // mov al, 'L'
      0xee,                         // out dx, al
      0xb0, 0x6f,                   // mov al, 'o'
      0xee,                         // out dx, al
      0xb0, 0x6e,                   // mov al, 'n'
      0xee,                         // out dx, al
      0xb0, 0x67,                   // mov al, 'g'
      0xee,                         // out dx, al
      0xb0, 0x20,                   // mov al, ' '
      0xee,                         // out dx, al
      0xb0, 0x4d,                   // mov al, 'M'
      0xee,                         // out dx, al
      0xb0, 0x6f,                   // mov al, 'o'
      0xee,                         // out dx, al
      0xb0, 0x64,                   // mov al, 'd'
      0xee,                         // out dx, al
      0xb0, 0x65,                   // mov al, 'e'
      0xee,                         // out dx, al
      0xb0, 0x21,                   // mov al, '!'
      0xee,                         // out dx, al
      0xb0, 0x20,                   // mov al, ' '
      0xee,                         // out dx, al

      // Verify Virtio-MMIO magic value at physical address 0x10000000
      0x8b, 0x04, 0x25, 0x00, 0x00, 0x00, 0x10, // mov eax, [0x10000000]
      0x3d, 0x76, 0x69, 0x72, 0x74,             // cmp eax, 0x74726976 ("virt")
      0x74, 0x04,                               // je pass (+4 bytes)
      0xb0, 0x46,                               // fail: mov al, 'F'
      0xeb, 0x02,                               // jmp out (+2 bytes)
      0xb0, 0x50,                               // pass: mov al, 'P'
      0xee,                                     // out: out dx, al

      // Write newline
      0xb0, 0x0a, // mov al, '\n'
      0xee,       // out dx, al

      // Signal exit
      0xba, 0xf9, 0x03, 0x00, 0x00, // mov edx, 0x3f9
      0xb0, 0x00,                   // mov al, 0
      0xee                          // out dx, al
  };

  // Load payload to guest physical 0x100000 (1 MB)
  memcpy((uint8_t *)ram + 0x100000, code, sizeof(code));

  ret = pane_vm_vcpu_create(vm, 0);
  if (ret != 0) {
    fprintf(stderr, "Failed to create vCPU: %s\n", strerror(-ret));
    munmap(ram, ram_size);
    pane_vm_destroy(vm);
    return 1;
  }

  // Set up 64-bit Long Mode (Firecracker/MicroVM style)
  ret = pane_vm_setup_firecracker_mode(vm, 0, 0x100000);
  if (ret != 0) {
    fprintf(stderr, "Failed to set up Firecracker mode: %s\n", strerror(-ret));
    munmap(ram, ram_size);
    pane_vm_destroy(vm);
    return 1;
  }

  printf("Starting 64-bit MicroVM...\n");
  ret = pane_vm_vcpu_run(vm, 0);
  if (ret != 0) {
    fprintf(stderr, "VM run failed: %s\n", strerror(-ret));
  } else {
    printf("VM exited clean.\n");
  }

  clock_gettime(CLOCK_MONOTONIC, &end);
  double elapsed_ms = (end.tv_sec - start.tv_sec) * 1000.0 +
                      (end.tv_nsec - start.tv_nsec) / 1000000.0;
  printf("Total VMM startup & execution latency: %.3f ms\n", elapsed_ms);

  munmap(ram, ram_size);
  pane_vm_destroy(vm);

  if (elapsed_ms >= 5.0) {
    printf("WARNING: Boot time is %.3f ms (budget is < 5 ms)\n", elapsed_ms);
  } else {
    printf("SUCCESS: Boot time is within target limits (%.3f ms < 5 ms)\n",
           elapsed_ms);
  }

  return ret == 0 ? 0 : 1;
}
