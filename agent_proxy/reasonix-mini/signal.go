package main

import (
	"context"
	"os"
	"os/signal"
)

// signalContext cancels the agent ctx on SIGINT so Ctrl-C aborts in-flight
// provider requests and running bash commands (SPEC §3.4: ctx threads
// throughout). The second Ctrl-C is left to the default terminate behavior.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
