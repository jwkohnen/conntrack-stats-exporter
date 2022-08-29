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

	"github.com/jwkohnen/conntrack-stats-exporter/exporter/internal"
)

func (e *exporter) execInNetns(name string, fn func()) (err error) {
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
		return e.scrapeErrors.Count(
			name,
			internal.OpNetnsPrepare,
			fmt.Errorf("failed to open fd of target netns %q: %w", name, err),
		)
	}

	defer func() {
		if errClose := targetNs.Close(); errClose != nil {
			errCleanup := e.scrapeErrors.Count(name, internal.OpNetnsCleanup, errClose)

			var errNs *internal.Err
			if err == nil || errors.As(err, &errNs) && errCleanup.OpPriority(errNs) {
				err = errCleanup
			}
		}
	}()

	if targetNs > 0 {
		runtime.LockOSThread()

		originalNs, err = netns.Get()
		if err != nil {
			return e.scrapeErrors.Count(
				name,
				internal.OpNetnsPrepare,
				fmt.Errorf("failed to open fd of original netns: %w", err),
			)
		}

		defer func() {
			if errSetOrig := netns.Set(originalNs); errSetOrig != nil {
				errRestore := e.scrapeErrors.Count(
					name,
					internal.OpNetnsRestore,
					fmt.Errorf("failed to restore original netns: %w", errSetOrig),
				)

				var errNs *internal.Err
				if err == nil || errors.As(err, &errNs) && errRestore.OpPriority(errNs) {
					err = errRestore
				}
			}

			runtime.UnlockOSThread()

			if errClose := originalNs.Close(); errClose != nil {
				errCleanup := e.scrapeErrors.Count(
					name,
					internal.OpNetnsCleanup,
					fmt.Errorf("failed to close fd of original netns: %w", errClose),
				)

				var errNs *internal.Err
				if err == nil || errors.As(err, &errNs) && errCleanup.OpPriority(errNs) {
					// FIXME: handle non-myErr errors
					err = errCleanup
				}
			}
		}()

		err = netns.Set(targetNs)
		if err != nil {
			return e.scrapeErrors.Count(
				name,
				internal.OpNetnsEnter,
				fmt.Errorf("failed to enter target netns: %w", err),
			)
		}
	}

	fn()

	return nil
}
