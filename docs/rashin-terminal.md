# Rashin in the terminal

Rashin can answer a question and place proposed commands on the current prompt.
It never runs a command for you. You review the result and press Enter yourself.
Rashin is optional, and the shell integration does nothing when `rashin` is not
installed.

## Ask

Use the explicit Ask command:

```text
rashin ask why is my laptop not suspending
```

The command-line binary also accepts `rashin <question>` as an alias. In the
interactive Fish, Bash and Zsh adapters that shorter form is reserved for
putting a proposed command on the prompt.

Ask is read only. It can inspect the machine, explain what it finds and propose
commands. Work that needs to change files or system state can be moved to an
Agent session in the Rashin app.

The command sends the current directory and the shell hook's last command and
exit status as terminal context. Answer text streams to the terminal, followed
by any proposed command plan.

Useful forms:

```text
rashin ask -c what about the previous error
rashin ask --last
rashin ask --buffer -- replace spaces in these filenames
```

`-c` continues the last terminal Ask session. `--last` prints the last answer
again. `--buffer` prints only proposed commands, one per line, so a shell can put
them on its prompt.

## Shell prompt integration

Fish, Bash and Zsh ship a small interactive wrapper. Calling
`rashin <words>` through the wrapper runs:

```text
rashin ask --buffer -- <words>
```

The wrapper passes `RASHIN_LAST_CMD` and `RASHIN_LAST_STATUS`, then replaces the
current prompt buffer with stdout. Explicit subcommands such as `rashin ask`,
`rashin status`, `rashin index` and `rashin on` pass through to the command-line
binary. Nothing executes until you press Enter.

Press **Alt+R** to transmute the command currently on the prompt into a proposed
command. The same review rule applies. The hooks do not report executed commands
back to Rashin and do not create saved recipes.

## Lifecycle

Rashin is off by default. Turn it on and off with:

```text
rashin on
rashin off
rashin status
```

Turning it off stops the daemon and disables its socket and usage collector.
With Rashin off or absent, no process, socket, timer, watcher or shell probe is
left running.

See [rashin.md](rashin.md) for the full Ask, Agent and Index architecture.
