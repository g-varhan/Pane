#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/mount.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <fcntl.h>
#include <errno.h>

#define MAX_JSON_SIZE 65536
#define MAX_ARGS 256
#define MAX_ENV 256

// Helper to parse simple JSON string array
// Example: "entrypoint": [ "nginx", "-g", "daemon off;" ]
static int parse_string_array(const char *json, const char *key, char **out_arr, int max_items) {
    const char *key_pos = strstr(json, key);
    if (!key_pos) return 0;

    const char *colon = strchr(key_pos, ':');
    if (!colon) return 0;

    const char *bracket_start = strchr(colon, '[');
    if (!bracket_start) return 0;

    const char *bracket_end = strchr(bracket_start, ']');
    if (!bracket_end) return 0;

    int count = 0;
    const char *p = bracket_start + 1;
    while (p < bracket_end && count < max_items) {
        const char *q1 = strchr(p, '"');
        if (!q1 || q1 >= bracket_end) break;
        q1++; // past open quote

        const char *q2 = strchr(q1, '"');
        if (!q2 || q2 > bracket_end) break;

        size_t len = q2 - q1;
        char *str = malloc(len + 1);
        strncpy(str, q1, len);
        str[len] = '\0';

        out_arr[count++] = str;
        p = q2 + 1;
    }
    return count;
}

// Helper to parse simple JSON string value
// Example: "workdir": "/var/www"
static char *parse_string_value(const char *json, const char *key) {
    const char *key_pos = strstr(json, key);
    if (!key_pos) return NULL;

    const char *colon = strchr(key_pos, ':');
    if (!colon) return NULL;

    const char *q1 = strchr(colon, '"');
    if (!q1) return NULL;
    q1++; // past open quote

    const char *q2 = strchr(q1, '"');
    if (!q2) return NULL;

    size_t len = q2 - q1;
    char *str = malloc(len + 1);
    strncpy(str, q1, len);
    str[len] = '\0';
    return str;
}

int main() {
    printf("Pane VM init started (PID 1)...\n");

    // 1. Mount API filesystems
    if (mount("proc", "/proc", "proc", 0, NULL) < 0) {
        perror("mount /proc");
    }
    if (mount("sysfs", "/sys", "sysfs", 0, NULL) < 0) {
        perror("mount /sys");
    }
    if (mount("devtmpfs", "/dev", "devtmpfs", 0, NULL) < 0) {
        // Fallback to devfs if devtmpfs fails
        if (mount("udev", "/dev", "tmpfs", 0, "size=10M,mode=0755") < 0) {
            perror("mount /dev");
        }
    }

    // Create mountpoints if missing and mount them
    mkdir("/dev/pts", 0755);
    if (mount("devpts", "/dev/pts", "devpts", 0, NULL) < 0) {
        perror("mount /dev/pts");
    }
    mkdir("/dev/shm", 0755);
    if (mount("tmpfs", "/dev/shm", "tmpfs", 0, NULL) < 0) {
        perror("mount /dev/shm");
    }

    // 2. Start Pane vsock agent in the background
    printf("Starting vsock guest agent...\n");
    pid_t agent_pid = fork();
    if (agent_pid == 0) {
        // Child: run agent
        // Try standard path first
        char *agent_argv[] = {"/usr/sbin/pane-agent", NULL};
        execv(agent_argv[0], agent_argv);
        
        // Fallback paths
        agent_argv[0] = "/usr/bin/pane-agent";
        execv(agent_argv[0], agent_argv);
        agent_argv[0] = "/pane-agent";
        execv(agent_argv[0], agent_argv);
        
        perror("execve pane-agent failed");
        exit(1);
    } else if (agent_pid < 0) {
        perror("fork agent");
    }

    // 3. Read OCI configuration
    char *json_buf = malloc(MAX_JSON_SIZE);
    memset(json_buf, 0, MAX_JSON_SIZE);
    
    int fd = open("/oci-config.json", O_RDONLY);
    if (fd >= 0) {
        read(fd, json_buf, MAX_JSON_SIZE - 1);
        close(fd);
    } else {
        // Try alternate location
        fd = open("/etc/oci-config.json", O_RDONLY);
        if (fd >= 0) {
            read(fd, json_buf, MAX_JSON_SIZE - 1);
            close(fd);
        }
    }

    char *entrypoint[MAX_ARGS];
    char *cmd[MAX_ARGS];
    char *env[MAX_ENV];
    memset(entrypoint, 0, sizeof(entrypoint));
    memset(cmd, 0, sizeof(cmd));
    memset(env, 0, sizeof(env));

    int ep_count = parse_string_array(json_buf, "\"entrypoint\"", entrypoint, MAX_ARGS);
    int cmd_count = parse_string_array(json_buf, "\"cmd\"", cmd, MAX_ARGS);
    int env_count = parse_string_array(json_buf, "\"env\"", env, MAX_ENV);
    char *workdir = parse_string_value(json_buf, "\"workdir\"");
    char *user = parse_string_value(json_buf, "\"user\"");

    free(json_buf);

    // 4. Set environment variables
    for (int i = 0; i < env_count; i++) {
        putenv(env[i]);
    }
    // Ensure PATH is set if missing
    if (!getenv("PATH")) {
        putenv("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin");
    }

    // 5. Change to workdir if set
    if (workdir && strlen(workdir) > 0) {
        printf("Changing working directory to %s\n", workdir);
        if (chdir(workdir) < 0) {
            perror("chdir");
        }
    }

    // 6. Setup user (UID/GID)
    if (user && strlen(user) > 0) {
        uid_t uid = 0;
        gid_t gid = 0;
        if (sscanf(user, "%u:%u", &uid, &gid) == 2) {
            printf("Setting UID:GID to %u:%u\n", uid, gid);
            setgid(gid);
            setuid(uid);
        }
    }

    // 7. Compose final exec args
    char *exec_argv[MAX_ARGS * 2];
    int exec_argc = 0;
    
    for (int i = 0; i < ep_count; i++) {
        exec_argv[exec_argc++] = entrypoint[i];
    }
    for (int i = 0; i < cmd_count; i++) {
        exec_argv[exec_argc++] = cmd[i];
    }
    exec_argv[exec_argc] = NULL;

    if (exec_argc > 0) {
        printf("Executing entrypoint: %s\n", exec_argv[0]);
        execvp(exec_argv[0], exec_argv);
        perror("execvp container entrypoint");
    } else {
        printf("No entrypoint or command specified in OCI config.\n");
    }

    // 8. Fallback Shell
    printf("Dropping to emergency fallback shell...\n");
    char *shell_argv[] = {"/bin/sh", NULL};
    execvp(shell_argv[0], shell_argv);
    
    shell_argv[0] = "/bin/ash";
    execvp(shell_argv[0], shell_argv);

    perror("Failed to start fallback shell");
    
    // Reap zombie processes
    while (1) {
        int status;
        pid_t pid = wait(&status);
        if (pid < 0) {
            if (errno == ECHILD) {
                // No more children, sleep
                sleep(10);
                continue;
            }
            perror("wait");
            sleep(1);
        }
    }
    return 0;
}
