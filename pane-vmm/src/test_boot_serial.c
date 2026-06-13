#include "pane_vmm.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/ioctl.h>
#include <linux/kvm.h>
#include <asm/bootparam.h>

#ifndef E820_RAM
#define E820_RAM 1
#endif

int main(int argc, char **argv) {
    int use_payload = (argc < 2);
    uint8_t *bzimage_buf = NULL;
    uint64_t payload_size = 0;
    uint64_t setup_size = 0;
    off_t bzimage_size = 0;

    if (!use_payload) {
        const char *kernel_path = argv[1];
        int kernel_fd = open(kernel_path, O_RDONLY);
        if (kernel_fd < 0) {
            perror("Failed to open kernel");
            return 1;
        }

        bzimage_size = lseek(kernel_fd, 0, SEEK_END);
        lseek(kernel_fd, 0, SEEK_SET);

        bzimage_buf = malloc(bzimage_size);
        if (!bzimage_buf) {
            perror("Failed to allocate kernel buffer");
            close(kernel_fd);
            return 1;
        }

        if (read(kernel_fd, bzimage_buf, bzimage_size) != bzimage_size) {
            perror("Failed to read kernel");
            free(bzimage_buf);
            close(kernel_fd);
            return 1;
        }
        close(kernel_fd);

        printf("Kernel size: %ld bytes\n", (long)bzimage_size);

        uint8_t setup_sects = bzimage_buf[0x1f1];
        if (setup_sects == 0) {
            setup_sects = 4;
        }
        setup_size = (setup_sects + 1) * 512;
        if (setup_size >= (uint64_t)bzimage_size) {
            fprintf(stderr, "Invalid bzImage: setup_size exceeds file size\n");
            free(bzimage_buf);
            return 1;
        }
        payload_size = bzimage_size - setup_size;
        printf("Setup size: %lu, Payload size: %lu\n", setup_size, payload_size);
    } else {
        printf("No kernel image specified. Running embedded bare-metal test payload in Real Mode...\n");
    }

    pane_vm_t *vm = NULL;
    int ret = pane_vm_create(&vm);
    if (ret != 0) {
        fprintf(stderr, "Failed to create VM: %s\n", strerror(-ret));
        if (bzimage_buf) free(bzimage_buf);
        return 1;
    }

    uint64_t ram_size = use_payload ? (64ULL * 1024) : (128ULL * 1024 * 1024);
    uint64_t mmio_addr = use_payload ? 0x20000 : 0x10000000;

    void *ram = mmap(NULL, ram_size, PROT_READ | PROT_WRITE, MAP_ANONYMOUS | MAP_PRIVATE, -1, 0);
    if (ram == MAP_FAILED) {
        perror("mmap RAM");
        pane_vm_destroy(vm);
        if (bzimage_buf) free(bzimage_buf);
        return 1;
    }

    ret = pane_vm_set_user_memory_region(vm, 0, 0, ram_size, (uint64_t)ram, 0);
    if (ret != 0) {
        fprintf(stderr, "Failed to set RAM region: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        if (bzimage_buf) free(bzimage_buf);
        return 1;
    }

    ret = pane_vm_init_irqchip(vm);
    if (ret != 0) {
        fprintf(stderr, "Failed to initialize IRQ chip: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        if (bzimage_buf) free(bzimage_buf);
        return 1;
    }

    ret = pane_vm_setup_virtio_mmio(vm, mmio_addr, 512, 5);
    if (ret != 0) {
        fprintf(stderr, "Failed to set up Virtio-MMIO: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        if (bzimage_buf) free(bzimage_buf);
        return 1;
    }

    if (!use_payload) {
        memcpy((uint8_t *)ram + 0x100000, bzimage_buf + setup_size, payload_size);

        struct boot_params *bp = (struct boot_params *)((uint8_t *)ram + 0x10000);
        memset(bp, 0, sizeof(struct boot_params));
        memcpy((uint8_t *)bp + 0x1f1, bzimage_buf + 0x1f1, 0x100);

        bp->hdr.type_of_loader = 0xFF;
        bp->hdr.loadflags |= 1;

        const char *cmdline = "console=none earlyprintk=serial virtio_mmio.device=512@0x10000000:5 root=/dev/ram0 rw";
        strcpy((char *)ram + 0x20000, cmdline);
        bp->hdr.cmd_line_ptr = 0x20000;
        bp->hdr.cmdline_size = strlen(cmdline) + 1;

        bp->e820_entries = 2;
        bp->e820_table[0].addr = 0x00000000;
        bp->e820_table[0].size = 0x0009f000;
        bp->e820_table[0].type = E820_RAM;

        bp->e820_table[1].addr = 0x00100000;
        bp->e820_table[1].size = ram_size - 0x00100000;
        bp->e820_table[1].type = E820_RAM;

        free(bzimage_buf);
    } else {
        uint8_t code[] = {
            0xfa,                         // cli (disable interrupts)
            0xba, 0xf8, 0x03,             // mov dx, 0x3f8
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
            0xb8, 0x00, 0x20,             // mov ax, 0x2000 (segment for physical 0x20000)
            0x8e, 0xd8,                   // mov ds, ax
            0x66, 0xa1, 0x00, 0x00,       // mov eax, [0] (reads from physical 0x20000)
            0x66, 0x3d, 0x76, 0x69, 0x72, 0x74, // cmp eax, 0x74726976 ("virt")
            0x74, 0x04,                   // je pass (+4 bytes, skipping fail path)
            0xb0, 0x46,                   // mov al, 'F'
            0xeb, 0x02,                   // jmp out (+2 bytes)
            0xb0, 0x50,                   // pass: mov al, 'P'
            0xee,                         // out: out dx, al (print result to 0x3f8)
            0x66, 0xba, 0xf9, 0x03,       // mov dx, 0x3f9 (exit signal port)
            0xb0, 0x00,                   // mov al, 0 (exit code 0 = success)
            0xee,                         // out dx, al (triggers clean VM exit)
        };
        memcpy((uint8_t *)ram, code, sizeof(code));
    }

    ret = pane_vm_vcpu_create(vm, 0);
    if (ret != 0) {
        fprintf(stderr, "Failed to create vCPU: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        return 1;
    }

    struct kvm_regs regs = {
        .rip = 0,
        .rsi = use_payload ? 0 : 0x10000,
        .rflags = 2,
    };
    if (!use_payload) {
        regs.rip = 0x100000;
    }
    ret = pane_vm_vcpu_set_regs(vm, 0, &regs);
    if (ret != 0) {
        fprintf(stderr, "Failed to set vCPU regs: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        return 1;
    }

    struct kvm_sregs sregs;
    ret = pane_vm_vcpu_get_sregs(vm, 0, &sregs);
    if (ret != 0) {
        fprintf(stderr, "Failed to get vCPU sregs: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        return 1;
    }

    if (!use_payload) {
        struct kvm_segment code_seg = {
            .base = 0,
            .limit = 0xffffffff,
            .selector = 0x8,
            .type = 11,
            .s = 1,
            .dpl = 0,
            .present = 1,
            .avl = 0,
            .l = 0,
            .db = 1,
            .g = 1,
        };
        struct kvm_segment data_seg = {
            .base = 0,
            .limit = 0xffffffff,
            .selector = 0x10,
            .type = 3,
            .s = 1,
            .dpl = 0,
            .present = 1,
            .avl = 0,
            .l = 0,
            .db = 1,
            .g = 1,
        };

        sregs.cs = code_seg;
        sregs.ds = data_seg;
        sregs.es = data_seg;
        sregs.fs = data_seg;
        sregs.gs = data_seg;
        sregs.ss = data_seg;

        sregs.cr0 |= 0x11; // Enable PE (0x1) and ET (0x10)
    } else {
        sregs.cs.base = 0;
        sregs.cs.selector = 0;
        sregs.ds.base = 0;
        sregs.ds.selector = 0;
        sregs.es.base = 0;
        sregs.es.selector = 0;
        sregs.ss.base = 0;
        sregs.ss.selector = 0;
    }

    ret = pane_vm_vcpu_set_sregs(vm, 0, &sregs);
    if (ret != 0) {
        fprintf(stderr, "Failed to set vCPU sregs: %s\n", strerror(-ret));
        munmap(ram, ram_size);
        pane_vm_destroy(vm);
        return 1;
    }

    printf("Starting VM...\n");
    ret = pane_vm_vcpu_run(vm, 0);
    if (ret != 0) {
        fprintf(stderr, "VM run failed: %s\n", strerror(-ret));
    } else {
        printf("\nVM exited clean.\n");
    }

    munmap(ram, ram_size);
    pane_vm_destroy(vm);
    return ret == 0 ? 0 : 1;
}
