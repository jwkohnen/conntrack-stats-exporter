//    This file is part of conntrack-stats-exporter.
//
//    conntrack-stats-exporter is free software: you can redistribute it and/or
//    modify it under the terms of the GNU General Public License as published
//    by the Free Software Foundation, either version 3 of the License, or (at
//    your option) any later version.
//
//    conntrack-stats-exporter is distributed in the hope that it will be
//    useful, but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
//    Public License for more details.
//
//    You should have received a copy of the GNU General Public License along
//    with conntrack-stats-exporter.  If not, see
//    <http://www.gnu.org/licenses/>.

package exporter

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/vishvananda/netns"
)

type ErrNetNs struct {
	op  string
	err error
}

const (
	opRestore = "restore"
	opEnter   = "enter"
	opCleanup = "cleanup"
	opPrepare = "prepare"
)

func opPriority(op string) int {
	switch op {
	case opRestore:
		return 1
	case opEnter:
		return 2
	case opCleanup:
		return 3
	case opPrepare:
		return 4
	default:
		panic(fmt.Sprintf("unknown op: %q", op))
	}
}

func (e ErrNetNs) Error() string        { return fmt.Sprintf("exec_in_ns: op(%s): %v", e.op, e.err) }
func (e ErrNetNs) Unwrap() error        { return e.err }
func (e ErrNetNs) isRestoreError() bool { return e.op == opRestore }
func (e ErrNetNs) isEnterError() bool   { return e.op == opEnter }

func execInNetns(name string, fn func()) (err error) {
	if name == "" {
		fn()

		return nil
	}

	var (
		targetNs   netns.NsHandle
		originalNs netns.NsHandle
	)

	targetNs, err = netns.GetFromName(name)
	if err != nil {
		return ErrNetNs{op: opPrepare, err: fmt.Errorf("failed to open fd of target netns %q: %w", name, err)}
	}

	defer func() {
		if errClose := targetNs.Close(); errClose != nil {
			var errNs *ErrNetNs
			if err == nil || errors.As(err, &errNs) && opPriority(errNs.op) < opPriority(opCleanup) {
				err = ErrNetNs{
					op:  opCleanup,
					err: fmt.Errorf("failed to close fd of target netns %q: %w", name, errClose),
				}
			}
		}
	}()

	if targetNs > 0 {
		runtime.LockOSThread()

		originalNs, err = netns.Get()
		if err != nil {
			return ErrNetNs{op: opPrepare, err: fmt.Errorf("failed to open fd of original netns: %w", err)}
		}

		defer func() {
			if errRestore := netns.Set(originalNs); errRestore != nil {
				var errNs *ErrNetNs
				if err == nil || errors.As(err, &errNs) && opPriority(errNs.op) < opPriority(opRestore) {
					err = ErrNetNs{
						op:  opRestore,
						err: fmt.Errorf("failed to restore original netns: %w", errRestore),
					}
				}
			}

			runtime.UnlockOSThread()

			if errClose := originalNs.Close(); errClose != nil {
				var errNs *ErrNetNs
				if err == nil || errors.As(err, &errNs) && opPriority(errNs.op) < opPriority(opCleanup) {
					err = ErrNetNs{
						op:  opCleanup,
						err: fmt.Errorf("failed to close fd of original netns: %w", errClose),
					}
				}
			}
		}()

		err = netns.Set(targetNs)
		if err != nil {
			return ErrNetNs{op: opEnter, err: fmt.Errorf("failed to enter target netns %q: %w", name, err)}
		}
	}

	fn()

	return nil
}
