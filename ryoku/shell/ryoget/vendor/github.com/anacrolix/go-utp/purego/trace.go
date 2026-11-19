package purego

import (
	"fmt"
	"os"
)

// Set to true and rebuild to dump every packet sent and received to stderr. It's a compile time
// constant so the tracing costs nothing when it's off, which matters on a path that runs per
// packet.
const traceEnabled = false

func trace(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
