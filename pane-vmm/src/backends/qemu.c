#include "../pane_vmm_internal.h"
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <signal.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>

static int send_qmp_cmd(int fd, const char *cmd, char *resp_out, size_t max_len) {
    ssize_t len = write(fd, cmd, strlen(cmd));
    if (len < 0) return -errno;

    while (1) {
        size_t total = 0;
        while (total < max_len - 1) {
            char c;
            ssize_t r = read(fd, &c, 1);
            if (r < 0) {
                if (errno == EINTR) continue;
                return -errno;
            }
            if (r == 0) break; // EOF
            resp_out[total++] = c;
            if (c == '\n') break;
        }
        resp_out[total] = '\0';

        // If this is an asynchronous event (e.g. {"event":"STOP"}), skip it and wait for command response
        if (strstr(resp_out, "\"event\":") == NULL) {
            break;
        }
    }
    return 0;
}

static int read_qmp_greeting(int fd, char *buf, size_t max_len) {
    size_t total = 0;
    while (total < max_len - 1) {
        char c;
        ssize_t r = read(fd, &c, 1);
        if (r < 0) {
            if (errno == EINTR) continue;
            return -errno;
        }
        if (r == 0) break;
        buf[total++] = c;
        if (c == '\n') break;
    }
    buf[total] = '\0';
    return 0;
}

int pane_vm_setup_qemu_mode(pane_vm_t *vm, const char *image_path, const char *qmp_socket_path) {
    if (!vm || !image_path || !qmp_socket_path) return -EINVAL;

    // Transition backend type
    vm->backend = PANE_BACKEND_QEMU;
    vm->qmp_path = strdup(qmp_socket_path);

    // Close native KVM resources
    if (vm->vm_fd >= 0) {
        close(vm->vm_fd);
        vm->vm_fd = -1;
    }
    if (vm->kvm_fd >= 0) {
        close(vm->kvm_fd);
        vm->kvm_fd = -1;
    }

    // Fork and execute qemu
    pid_t pid = fork();
    if (pid < 0) {
        return -errno;
    }

    if (pid == 0) {
        // Child process: execute QEMU
        char *args[] = {
            "qemu-system-x86_64",
            "-enable-kvm",
            "-m", "128",
            "-smp", "1",
            "-display", "none",
            "-nographic",
            "-drive", NULL, // Will format below
            "-qmp", NULL,   // Will format below
            "-serial", "none",
            NULL
        };

        // Formats
        char drive_arg[512];
        snprintf(drive_arg, sizeof(drive_arg), "file=%s,format=raw,if=virtio", image_path);
        args[10] = drive_arg;

        char qmp_arg[512];
        snprintf(qmp_arg, sizeof(qmp_arg), "unix:%s,server,nowait", qmp_socket_path);
        args[12] = qmp_arg;

        execvp("qemu-system-x86_64", args);
        // If execvp returns, it failed
        perror("execvp qemu-system-x86_64");
        exit(1);
    }

    // Parent process
    vm->qemu_pid = pid;

    // Connect to QMP Unix socket with retries (allow QEMU to initialize socket listener)
    int client_fd = -1;
    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, qmp_socket_path, sizeof(addr.sun_path) - 1);

    int retries = 40; // 40 * 50ms = 2 seconds max startup time
    while (retries > 0) {
        client_fd = socket(AF_UNIX, SOCK_STREAM, 0);
        if (client_fd < 0) {
            return -errno;
        }

        if (connect(client_fd, (struct sockaddr *)&addr, sizeof(addr)) == 0) {
            break; // Connected!
        }

        close(client_fd);
        client_fd = -1;
        usleep(50000); // Wait 50ms
        retries--;
    }

    if (client_fd < 0) {
        fprintf(stderr, "Failed to connect to QMP socket at %s\n", qmp_socket_path);
        kill(pid, SIGKILL);
        waitpid(pid, NULL, 0);
        return -ECONNREFUSED;
    }

    vm->qmp_fd = client_fd;

    // Read greeting
    char buf[1024];
    int ret = read_qmp_greeting(client_fd, buf, sizeof(buf));
    if (ret < 0) {
        close(client_fd);
        vm->qmp_fd = -1;
        return ret;
    }

    // Perform capabilities handshake
    ret = send_qmp_cmd(client_fd, "{\"execute\":\"qmp_capabilities\"}\n", buf, sizeof(buf));
    if (ret < 0) {
        close(client_fd);
        vm->qmp_fd = -1;
        return ret;
    }

    // Verify response
    if (strstr(buf, "\"return\"") == NULL) {
        fprintf(stderr, "QMP handshake failed, got: %s\n", buf);
        close(client_fd);
        vm->qmp_fd = -1;
        return -EPROTO;
    }

    return 0;
}

int pane_vm_qemu_suspend(pane_vm_t *vm) {
    if (!vm || vm->backend != PANE_BACKEND_QEMU || vm->qmp_fd < 0) return -EINVAL;

    char buf[1024];
    int ret = send_qmp_cmd(vm->qmp_fd, "{\"execute\":\"stop\"}\n", buf, sizeof(buf));
    if (ret < 0) return ret;

    if (strstr(buf, "\"return\"") == NULL) return -EPROTO;
    return 0;
}

int pane_vm_qemu_resume(pane_vm_t *vm) {
    if (!vm || vm->backend != PANE_BACKEND_QEMU || vm->qmp_fd < 0) return -EINVAL;

    char buf[1024];
    int ret = send_qmp_cmd(vm->qmp_fd, "{\"execute\":\"cont\"}\n", buf, sizeof(buf));
    if (ret < 0) return ret;

    if (strstr(buf, "\"return\"") == NULL) return -EPROTO;
    return 0;
}

static int parse_status(const char *json, char *status_out, size_t max_len) {
    const char *key = "\"status\": \"";
    const char *pos = strstr(json, key);
    if (!pos) return -1;
    pos += strlen(key);
    size_t i = 0;
    while (*pos && *pos != '"' && i < max_len - 1) {
        status_out[i++] = *pos++;
    }
    status_out[i] = '\0';
    return 0;
}

int pane_vm_qemu_query_status(pane_vm_t *vm, char *status_out, size_t max_len) {
    if (!vm || vm->backend != PANE_BACKEND_QEMU || vm->qmp_fd < 0 || !status_out) return -EINVAL;

    char buf[1024];
    int ret = send_qmp_cmd(vm->qmp_fd, "{\"execute\":\"query-status\"}\n", buf, sizeof(buf));
    if (ret < 0) return ret;

    if (strstr(buf, "\"return\"") == NULL) return -EPROTO;

    if (parse_status(buf, status_out, max_len) < 0) {
        return -EPROTO;
    }

    return 0;
}
