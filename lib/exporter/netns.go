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
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

func execInNetns(name string, fn func() error) error {
	if name == "" {
		return fn()
	}

	ns, err := netns.GetFromName(name)
	if err != nil {
		return err
	}
	defer func() {
		err := ns.Close()
		if err != nil {
			log.Error("exec_in_ns: failed to close fd: %w", err)
		}
	}()

	if ns > 0 {
		runtime.LockOSThread()
		current, err := netns.Get()
		if err != nil {
			return err
		}

		defer func() {
			err := netns.Set(current)
			if err != nil {
				log.Error("exec_in_ns: failed to restore netns: %w", err)
			}
			runtime.UnlockOSThread()
			err = current.Close()
			if err != nil {
				log.Error("exec_in_ns: failed to close fd: %w", err)
			}
		}()

		err = netns.Set(ns)
		if err != nil {
			return err
		}
	}

	return fn()
}
