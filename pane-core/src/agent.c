#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <netinet/in.h>
#include <linux/vm_sockets.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <fcntl.h>
#include <poll.h>
#include <errno.h>
#include <stdint.h>

#define MAX_JSON_SIZE 65536
#define MAX_ARGS 512
#define BUFFER_SIZE 4096

// Frame types
#define FRAME_STDOUT 1
#define FRAME_STDERR 2
#define FRAME_EXIT_CODE 3

// Helper to write all bytes to a file descriptor
static ssize_t write_all(int fd, const void *buf, size_t count) {
    size_t total = 0;
    const uint8_t *p = buf;
    while (total < count) {
        ssize_t r = write(fd, p + total, count - total);
        if (r < 0) {
            if (errno == EINTR) continue;
            return r;
        }
        total += r;
    }
    return total;
}

// Writes a framed message to the client socket
static int write_frame(int sock, uint8_t type, const void *buf, uint32_t len) {
    uint8_t header[5];
    header[0] = type;
    header[1] = (len >> 24) & 0xFF;
    header[2] = (len >> 16) & 0xFF;
    header[3] = (len >> 8) & 0xFF;
    header[4] = len & 0xFF;

    if (write_all(sock, header, 5) != 5) return -1;
    if (len > 0 && write_all(sock, buf, len) != (ssize_t)len) return -1;
    return 0;
}

// Extremely simple JSON parser to extract "command" and "args"
// Returns 0 on success, -1 on failure
static int parse_request(const char *json, char *command, size_t max_cmd_len, char **args, int *arg_count) {
    *arg_count = 0;
    
    // Find "command"
    const char *cmd_key = strstr(json, "\"command\"");
    if (!cmd_key) return -1;
    
    const char *colon = strchr(cmd_key, ':');
    if (!colon) return -1;
    
    const char *start_quote = strchr(colon, '"');
    if (!start_quote) return -1;
    start_quote++; // move past quote
    
    const char *end_quote = strchr(start_quote, '"');
    if (!end_quote) return -1;
    
    size_t cmd_len = end_quote - start_quote;
    if (cmd_len >= max_cmd_len) cmd_len = max_cmd_len - 1;
    strncpy(command, start_quote, cmd_len);
    command[cmd_len] = '\0';
    
    // First arg is the command itself
    args[0] = strdup(command);
    *arg_count = 1;

    // Find "args"
    const char *args_key = strstr(json, "\"args\"");
    if (args_key) {
        const char *args_colon = strchr(args_key, ':');
        if (args_colon) {
            const char *start_bracket = strchr(args_colon, '[');
            const char *end_bracket = strchr(args_colon, ']');
            if (start_bracket && end_bracket && start_bracket < end_bracket) {
                const char *p = start_bracket + 1;
                while (p < end_bracket && *arg_count < MAX_ARGS - 1) {
                    // Find next quote
                    const char *q1 = strchr(p, '"');
                    if (!q1 || q1 >= end_bracket) break;
                    q1++;
                    const char *q2 = strchr(q1, '"');
                    if (!q2 || q2 > end_bracket) break;
                    
                    size_t arg_len = q2 - q1;
                    char *arg = malloc(arg_len + 1);
                    strncpy(arg, q1, arg_len);
                    arg[arg_len] = '\0';
                    
                    args[*arg_count] = arg;
                    (*arg_count)++;
                    
                    p = q2 + 1;
                }
            }
        }
    }
    
    args[*arg_count] = NULL;
    return 0;
}

// Handles an incoming connection
static void handle_client(int client_fd) {
    char json_buf[MAX_JSON_SIZE];
    size_t total_read = 0;
    
    // Read JSON payload until EOF or buffer full
    while (total_read < MAX_JSON_SIZE - 1) {
        ssize_t r = read(client_fd, json_buf + total_read, MAX_JSON_SIZE - 1 - total_read);
        if (r < 0) {
            if (errno == EINTR) continue;
            perror("read client_fd");
            close(client_fd);
            return;
        }
        if (r == 0) break; // EOF
        total_read += r;
        
        // Quick exit if we already see the complete JSON structure (e.g. ends with '}')
        if (total_read > 0 && json_buf[total_read - 1] == '}') {
            // Check if braces are balanced (simple check)
            int open_braces = 0;
            for (size_t i = 0; i < total_read; i++) {
                if (json_buf[i] == '{') open_braces++;
                else if (json_buf[i] == '}') open_braces--;
            }
            if (open_braces == 0) {
                break;
            }
        }
    }
    json_buf[total_read] = '\0';

    char command[1024];
    char *args[MAX_ARGS];
    int arg_count = 0;
    
    if (parse_request(json_buf, command, sizeof(command), args, &arg_count) < 0) {
        const char *err = "Invalid JSON request envelope\n";
        write_frame(client_fd, FRAME_STDERR, err, strlen(err));
        uint32_t exit_code_be = (uint32_t)htonl(1);
        write_frame(client_fd, FRAME_EXIT_CODE, &exit_code_be, 4);
        close(client_fd);
        return;
    }
    
    // Create pipes for stdout and stderr
    int stdout_pipe[2];
    int stderr_pipe[2];
    if (pipe(stdout_pipe) < 0 || pipe(stderr_pipe) < 0) {
        perror("pipe");
        close(client_fd);
        return;
    }
    
    pid_t pid = fork();
    if (pid < 0) {
        perror("fork");
        close(client_fd);
        return;
    }
    
    if (pid == 0) {
        // Child process
        close(stdout_pipe[0]);
        close(stderr_pipe[0]);
        
        dup2(stdout_pipe[1], STDOUT_FILENO);
        dup2(stderr_pipe[1], STDERR_FILENO);
        
        close(stdout_pipe[1]);
        close(stderr_pipe[1]);
        close(client_fd);
        
        execvp(command, args);
        // If execvp fails
        fprintf(stderr, "Failed to exec %s: %s\n", command, strerror(errno));
        exit(127);
    }
    
    // Parent process
    close(stdout_pipe[1]);
    close(stderr_pipe[1]);
    
    // Set non-blocking on read ends of pipes
    fcntl(stdout_pipe[0], F_SETFL, O_NONBLOCK);
    fcntl(stderr_pipe[0], F_SETFL, O_NONBLOCK);
    
    struct pollfd fds[2];
    fds[0].fd = stdout_pipe[0];
    fds[0].events = POLLIN;
    fds[1].fd = stderr_pipe[0];
    fds[1].events = POLLIN;
    
    char read_buf[BUFFER_SIZE];
    int active_fds = 2;
    
    while (active_fds > 0) {
        int ret = poll(fds, 2, -1);
        if (ret < 0) {
            if (errno == EINTR) continue;
            break;
        }
        
        for (int i = 0; i < 2; i++) {
            if (fds[i].fd == -1) continue;
            
            if (fds[i].revents & (POLLIN | POLLHUP | POLLERR)) {
                ssize_t r = read(fds[i].fd, read_buf, BUFFER_SIZE);
                if (r < 0) {
                    if (errno == EAGAIN || errno == EWOULDBLOCK) continue;
                    perror("read pipe");
                    close(fds[i].fd);
                    fds[i].fd = -1;
                    active_fds--;
                } else if (r == 0) {
                    // EOF
                    close(fds[i].fd);
                    fds[i].fd = -1;
                    active_fds--;
                } else {
                    // Write frame to client
                    uint8_t type = (i == 0) ? FRAME_STDOUT : FRAME_STDERR;
                    if (write_frame(client_fd, type, read_buf, r) < 0) {
                        // Client disconnected
                        close(stdout_pipe[0]);
                        close(stderr_pipe[0]);
                        kill(pid, SIGTERM);
                        close(client_fd);
                        return;
                    }
                }
            }
        }
    }
    
    // Clean up duplicated args
    for (int i = 0; i < arg_count; i++) {
        free(args[i]);
    }
    
    // Wait for child exit code
    int status = 0;
    int exit_code = 0;
    if (waitpid(pid, &status, 0) == pid) {
        if (WIFEXITED(status)) {
            exit_code = WEXITSTATUS(status);
        } else if (WIFSIGNALED(status)) {
            exit_code = 128 + WTERMSIG(status);
        }
    }
    
    uint32_t exit_code_be = (uint32_t)htonl(exit_code);
    write_frame(client_fd, FRAME_EXIT_CODE, &exit_code_be, 4);
    close(client_fd);
}

int main(int argc, char **argv) {
    int server_fd = -1;
    
    if (argc >= 3 && strcmp(argv[1], "--uds") == 0) {
        const char *path = argv[2];
        unlink(path);
        
        server_fd = socket(AF_UNIX, SOCK_STREAM, 0);
        if (server_fd < 0) {
            perror("socket AF_UNIX");
            return 1;
        }
        
        struct sockaddr_un addr;
        memset(&addr, 0, sizeof(addr));
        addr.sun_family = AF_UNIX;
        strncpy(addr.sun_path, path, sizeof(addr.sun_path) - 1);
        
        if (bind(server_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
            perror("bind AF_UNIX");
            return 1;
        }
    } else if (argc >= 3 && strcmp(argv[1], "--tcp") == 0) {
        int port = atoi(argv[2]);
        server_fd = socket(AF_INET, SOCK_STREAM, 0);
        if (server_fd < 0) {
            perror("socket AF_INET");
            return 1;
        }
        
        int opt = 1;
        setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
        
        struct sockaddr_in addr;
        memset(&addr, 0, sizeof(addr));
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = INADDR_ANY;
        addr.sin_port = htons(port);
        
        if (bind(server_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
            perror("bind AF_INET");
            return 1;
        }
    } else {
        // Default: AF_VSOCK port 1024
        server_fd = socket(AF_VSOCK, SOCK_STREAM, 0);
        if (server_fd < 0) {
            perror("socket AF_VSOCK");
            return 1;
        }
        
        struct sockaddr_vm addr;
        memset(&addr, 0, sizeof(addr));
        addr.svm_family = AF_VSOCK;
        addr.svm_cid = VMADDR_CID_ANY;
        addr.svm_port = 1024;
        
        if (bind(server_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
            perror("bind AF_VSOCK");
            return 1;
        }
    }
    
    if (listen(server_fd, 128) < 0) {
        perror("listen");
        return 1;
    }
    
    while (1) {
        int client_fd = accept(server_fd, NULL, NULL);
        if (client_fd < 0) {
            if (errno == EINTR) continue;
            perror("accept");
            break;
        }
        
        handle_client(client_fd);
    }
    
    close(server_fd);
    return 0;
}
