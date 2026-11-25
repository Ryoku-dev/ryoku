// SPDX-License-Identifier: GPL-3.0-only
// refuse.c: the lab's stand-in for systemctl, ryoku, ryoku-shell and
// ryoku-host (up.sh copies one build under each name). It logs the refused
// command to REFUSED_LOG and exits 1. It is a program, not a script: /proc
// then shows the command line the daemon ran, where a script's would be its
// interpreter's ("/bin/sh /path/ryoku wm caps").
#include <limits.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#ifndef REFUSED_LOG
#error "build with -DREFUSED_LOG='\"/path/refused.log\"'"
#endif

int main(int argc, char **argv) {
    // the name it was installed under, whatever argv[0] the caller passed
    char exe[PATH_MAX];
    ssize_t n = readlink("/proc/self/exe", exe, sizeof exe - 1);
    exe[n > 0 ? n : 0] = '\0';
    const char *name = strrchr(exe, '/');
    name = name ? name + 1 : (argc > 0 ? argv[0] : "?");
    FILE *log = fopen(REFUSED_LOG, "a");
    if (log) {
        fprintf(log, "refused: %s ", name);
        for (int i = 1; i < argc; i++)
            fprintf(log, i > 1 ? " %s" : "%s", argv[i]);
        fputc('\n', log);
        fclose(log);
    }
    return 1;
}
