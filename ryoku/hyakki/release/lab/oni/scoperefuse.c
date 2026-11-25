// SPDX-License-Identifier: GPL-3.0-only
// scoperefuse.c: the lab's stand-in for systemd-run (bin/systemd-run). A
// reflex launch on a systemd host wraps the app as
// "systemd-run --user --scope --collect -- EXEC...": the real tool would ask
// the live user manager for a scope, so the lab refuses it the way the real
// tool fails when the manager says no. It never runs the wrapped command,
// logs the refused command line to REFUSED_LOG in refuse.c's format
// ("refused: systemd-run ARGS"), prints systemd-run's own failure line on
// stderr and exits 1, so the launcher's fallback (a detached start recorded
// as scope=fallback) is what the lab sees. Like refuse.c it is a program,
// not a script, so /proc shows the command line the daemon ran.
#include <stdio.h>

#ifndef REFUSED_LOG
#error "build with -DREFUSED_LOG='\"/path/refused.log\"'"
#endif

int main(int argc, char **argv) {
    FILE *log = fopen(REFUSED_LOG, "a");
    if (log) {
        fputs("refused: systemd-run ", log);
        for (int i = 1; i < argc; i++)
            fprintf(log, i > 1 ? " %s" : "%s", argv[i]);
        fputc('\n', log);
        fclose(log);
    }
    fputs("Failed to start transient scope unit: Access denied\n", stderr);
    return 1;
}
