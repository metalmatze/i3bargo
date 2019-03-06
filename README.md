# i3bargo

This is a status bar for i3 written in Go.

The main improvements compared to other projects are
better utilization of channels to send the computed JSON to
the main goroutine.

## Architecture

The state of the different blocks is computed in goroutines that sleep
a specific time and then share their state by sending to a channel.

In the main goroutine these JSON parts are combined and a full array
of objects is printed to stdout.