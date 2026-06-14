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

// Dynamic argv builder structure
typedef struct {
    char **argv;
    int argc;
    int capacity;
} qemu_argv_t;

static qemu_argv_t *qemu_argv_new(void) {
    qemu_argv_t *qa = malloc(sizeof(qemu_argv_t));
    if (!qa) return NULL;
    qa->argc = 0;
    qa->capacity = 16;
    qa->argv = malloc(sizeof(char*) * qa->capacity);
    if (!qa->argv) {
        free(qa);
        return NULL;
    }
    qa->argv[0] = NULL;
    return qa;
}

static void qemu_argv_push(qemu_argv_t *qa, const char *arg) {
    if (!qa || !arg) return;
    if (qa->argc >= qa->capacity - 2) {
        qa->capacity *= 2;
        char **new_argv = realloc(qa->argv, sizeof(char*) * qa->capacity);
        if (!new_argv) return;
        qa->argv = new_argv;
    }
    qa->argv[qa->argc++] = strdup(arg);
    qa->argv[qa->argc] = NULL;
}

static void qemu_argv_free(qemu_argv_t *qa) {
    if (!qa) return;
    for (int i = 0; i < qa->argc; i++) {
        free(qa->argv[i]);
    }
    free(qa->argv);
    free(qa);
}

// Helpers for composition
static void qemu_args_add_machine(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    (void)cfg;
    qemu_argv_push(qa, "qemu-system-x86_64");
    qemu_argv_push(qa, "-enable-kvm");
}

static void qemu_args_add_cpu_mem(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    char mem_str[64];
    uint64_t mib = cfg->memory_bytes / (1024 * 1024);
    if (mib == 0) mib = 128; // default
    snprintf(mem_str, sizeof(mem_str), "%lu", mib);
    qemu_argv_push(qa, "-m");
    qemu_argv_push(qa, mem_str);

    char cpu_str[64];
    uint32_t cpus = cfg->vcpus;
    if (cpus == 0) cpus = 1;
    snprintf(cpu_str, sizeof(cpu_str), "%u", cpus);
    qemu_argv_push(qa, "-smp");
    qemu_argv_push(qa, cpu_str);
}

static void qemu_args_add_disk(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    if (cfg->disk_path && strlen(cfg->disk_path) > 0) {
        const char *fmt = cfg->disk_format ? cfg->disk_format : "raw";
        char drive_arg[1024];
        snprintf(drive_arg, sizeof(drive_arg), "file=%s,format=%s,if=virtio", cfg->disk_path, fmt);
        qemu_argv_push(qa, "-drive");
        qemu_argv_push(qa, drive_arg);
    }
}

static void qemu_args_add_net(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    if (cfg->virtio_net) {
        const char *vm_id = cfg->vm_id ? cfg->vm_id : "default";
        char netdev_arg[1024];
        if (cfg->net_bridge && strlen(cfg->net_bridge) > 0) {
            snprintf(netdev_arg, sizeof(netdev_arg), "bridge,id=net0,br=%s", cfg->net_bridge);
        } else {
            snprintf(netdev_arg, sizeof(netdev_arg), "tap,id=net0,ifname=pane-%s-tap0,script=no,downscript=no", vm_id);
        }
        qemu_argv_push(qa, "-netdev");
        qemu_argv_push(qa, netdev_arg);
        qemu_argv_push(qa, "-device");
        qemu_argv_push(qa, "virtio-net-pci,netdev=net0");
    }
}

static void qemu_args_add_rng(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    if (cfg->virtio_rng) {
        qemu_argv_push(qa, "-device");
        qemu_argv_push(qa, "virtio-rng-pci");
    }
}

static void qemu_args_add_kernel(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    if (cfg->kernel_path && strlen(cfg->kernel_path) > 0) {
        qemu_argv_push(qa, "-kernel");
        qemu_argv_push(qa, cfg->kernel_path);
        if (cfg->cmdline && strlen(cfg->cmdline) > 0) {
            qemu_argv_push(qa, "-append");
            qemu_argv_push(qa, cfg->cmdline);
        }
    }
}

static void qemu_args_add_extra(qemu_argv_t *qa, const pane_vmm_config_t *cfg) {
    if (cfg->extra_args) {
        for (int i = 0; cfg->extra_args[i] != NULL; i++) {
            qemu_argv_push(qa, cfg->extra_args[i]);
        }
    }
}

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

        // Skip asynchronous events
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

int pane_vm_setup_qemu_mode(pane_vm_t *vm, const pane_vmm_config_t *config, const char *qmp_socket_path) {
    if (!vm || !config || !qmp_socket_path) return -EINVAL;

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

    // Build the dynamic QEMU command line
    qemu_argv_t *qa = qemu_argv_new();
    if (!qa) return -ENOMEM;

    qemu_args_add_machine(qa, config);
    qemu_args_add_cpu_mem(qa, config);
    qemu_args_add_disk(qa, config);
    qemu_args_add_net(qa, config);
    qemu_args_add_rng(qa, config);
    qemu_args_add_kernel(qa, config);
    
    // Add QMP Socket argument
    char qmp_arg[1024];
    snprintf(qmp_arg, sizeof(qmp_arg), "unix:%s,server,nowait", qmp_socket_path);
    qemu_argv_push(qa, "-qmp");
    qemu_argv_push(qa, qmp_arg);

    qemu_args_add_extra(qa, config);

    // Build the QEMU log file path from qmp_socket_path
    char log_path[1024];
    strncpy(log_path, qmp_socket_path, sizeof(log_path) - 1);
    char *last_slash = strrchr(log_path, '/');
    if (last_slash) {
        int dir_len = last_slash - log_path;
        snprintf(log_path, sizeof(log_path), "%.*s/qemu-%s.log", dir_len, qmp_socket_path, config->vm_id);
    } else {
        snprintf(log_path, sizeof(log_path), "qemu-%s.log", config->vm_id);
    }

    // Fork and execute qemu
    pid_t pid = fork();
    if (pid < 0) {
        qemu_argv_free(qa);
        return -errno;
    }

    if (pid == 0) {
        // Redirect stdout and stderr to the log file
        int log_fd = open(log_path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
        if (log_fd >= 0) {
            dup2(log_fd, 1);
            dup2(log_fd, 2);
            close(log_fd);
        } else {
            int dev_null = open("/dev/null", O_RDWR);
            if (dev_null >= 0) {
                dup2(dev_null, 1);
                dup2(dev_null, 2);
                close(dev_null);
            }
        }
        int dev_null_in = open("/dev/null", O_RDONLY);
        if (dev_null_in >= 0) {
            dup2(dev_null_in, 0);
            close(dev_null_in);
        }

        // Child process: execute QEMU
        execvp("qemu-system-x86_64", qa->argv);
        perror("execvp qemu-system-x86_64");
        exit(1);
    }

    // Parent process
    qemu_argv_free(qa);
    vm->qemu_pid = pid;

    // Connect to QMP Unix socket
    int client_fd = -1;
    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, qmp_socket_path, sizeof(addr.sun_path) - 1);

    int retries = 40;
    while (retries > 0) {
        client_fd = socket(AF_UNIX, SOCK_STREAM, 0);
        if (client_fd < 0) {
            return -errno;
        }

        if (connect(client_fd, (struct sockaddr *)&addr, sizeof(addr)) == 0) {
            break;
        }

        close(client_fd);
        client_fd = -1;
        usleep(50000);
        retries--;
    }

    if (client_fd < 0) {
        fprintf(stderr, "Failed to connect to QMP socket at %s\n", qmp_socket_path);
        FILE *log_file = fopen(log_path, "r");
        if (log_file) {
            fprintf(stderr, "QEMU log output:\n");
            char line[256];
            while (fgets(line, sizeof(line), log_file)) {
                fprintf(stderr, "  %s", line);
            }
            fclose(log_file);
        }
        kill(pid, SIGKILL);
        waitpid(pid, NULL, 0);
        return -ECONNREFUSED;
    }

    vm->qmp_fd = client_fd;

    char buf[1024];
    int ret = read_qmp_greeting(client_fd, buf, sizeof(buf));
    if (ret < 0) {
        close(client_fd);
        vm->qmp_fd = -1;
        return ret;
    }

    ret = send_qmp_cmd(client_fd, "{\"execute\":\"qmp_capabilities\"}\n", buf, sizeof(buf));
    if (ret < 0) {
        close(client_fd);
        vm->qmp_fd = -1;
        return ret;
    }

    if (strstr(buf, "\"return\"") == NULL) {
        fprintf(stderr, "QMP handshake failed, got: %s\n", buf);
        close(client_fd);
        vm->qmp_fd = -1;
        return -EPROTO;
    }

    close(client_fd);
    vm->qmp_fd = -1;
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
