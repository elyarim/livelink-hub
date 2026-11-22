//go:build !windows

package main

import "errors"

func autostartState() AutostartState {
	return AutostartState{On: false}
}

func setAutostart(on bool) error {
	return errors.New("on Linux, configure autostart via systemd (see install-linux.sh)")
}
