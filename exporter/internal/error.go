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

package internal

import (
	"fmt"
	"strconv"
	"sync"
)

type Err struct {
	op  op
	err error
}

type ScrapeErrors struct {
	mu sync.Mutex

	counts map[string]map[op]uint64
}

func (s *ScrapeErrors) Count(netns string, op op, err error) Err {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.init(netns, op)

	s.counts[netns][op]++

	return Err{
		op:  op,
		err: err,
	}
}

func (s *ScrapeErrors) Samples() Samples {
	s.mu.Lock()
	defer s.mu.Unlock()

	samples := make(Samples, 0, len(s.counts))

	for netns, causes := range s.counts {
		for cause, count := range causes {
			samples = append(
				samples,
				Sample{
					Labels: Labels{
						Label{
							Key:   "netns",
							Value: netns,
						},
						Label{
							Key:   "cause",
							Value: string(cause),
						},
					},
					Count: strconv.FormatUint(count, 10),
				},
			)
		}
	}

	return samples
}

func (s *ScrapeErrors) init(netns string, cause op) {
	if s.counts[netns] == nil {
		s.counts[netns] = make(map[op]uint64)
	}

	if _, ok := s.counts[netns][cause]; !ok {
		s.counts[netns][cause] = 0
	}
}

func NewScrapeErrors(netns []string) *ScrapeErrors {
	s := &ScrapeErrors{
		counts: make(map[string]map[op]uint64, len(netns)),
	}

	for _, ns := range netns {
		for _, cause := range []op{
			OpNetnsRestore,
			OpNetnsEnter,
			OpNetnsCleanup,
			OpNetnsPrepare,
			OpExecTool,
			OpToolOutputNoMatch,
			OpTimeout,
			OpClientGone,
		} {
			s.init(ns, cause)
		}
	}

	return s
}

type op string

const (
	OpNetnsRestore op = "netns_restore"
	OpNetnsEnter   op = "netns_enter"
	OpNetnsCleanup op = "netns_cleanup"
	OpNetnsPrepare op = "netns_prepare"

	OpExecTool          op = "tool_exec"
	OpToolOutputNoMatch op = "tool_output_no_match"
	OpTimeout           op = "timeout"
	OpClientGone        op = "client_gone"
)

func (e Err) OpPriority(other *Err) bool {
	prior := func(e Err) int {
		switch e.op {
		case OpNetnsRestore:
			return 1
		case OpNetnsEnter:
			return 2
		case OpNetnsCleanup:
			return 3
		case OpNetnsPrepare:
			return 4
		default:
			return 5
		}
	}

	return prior(e) < prior(*other)
}

func (e Err) Error() string { return fmt.Sprintf("op(%s): %v", e.op, e.err) }
func (e Err) Unwrap() error { return e.err }
func (e Err) label() Label  { return Label{Key: "cause", Value: string(e.op)} }

var _ = Err.label // TODO remove
