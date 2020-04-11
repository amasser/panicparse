// Copyright 2018 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate go get golang.org/x/tools/cmd/stringer
//go:generate stringer -type state

package stack

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Context is a parsing context.
//
// It contains the deduced GOROOT and GOPATH, if guesspaths is true.
type Context struct {
	// Goroutines is the Goroutines found.
	//
	// They are in the order that they were printed.
	Goroutines []*Goroutine

	// GOROOT is the GOROOT as detected in the traceback, not the on the host.
	//
	// It can be empty if no root was determined, for example the traceback
	// contains only non-stdlib source references.
	//
	// Empty is guesspaths was false.
	GOROOT string
	// GOPATHs is the GOPATH as detected in the traceback, with the value being
	// the corresponding path mapped to the host.
	//
	// It can be empty if only stdlib code is in the traceback or if no local
	// sources were matched up. In the general case there is only one entry in
	// the map.
	//
	// Nil is guesspaths was false.
	GOPATHs map[string]string

	// localGomoduleRoot is the root directory containing go.mod. It is
	// considered to be the primary project containing the main executable. It is
	// initialized by findRoots().
	//
	// It only works with stack traces created in the local file system.
	localGomoduleRoot string
	// gomodImportPath is set to the relative import path that localGomoduleRoot
	// represents.
	gomodImportPath string

	// localgoroot is GOROOT with "/" as path separator. No trailing "/".
	localgoroot string
	// localgopaths is GOPATH with "/" as path separator. No trailing "/".
	localgopaths []string
}

// ParseDump processes the output from runtime.Stack().
//
// Returns nil *Context if no stack trace was detected.
//
// It pipes anything not detected as a panic stack trace from r into out. It
// assumes there is junk before the actual stack trace. The junk is streamed to
// out.
//
// If guesspaths is false, no guessing of GOROOT and GOPATH is done, and Call
// entites do not have LocalSrcPath and IsStdlib filled in. If true, be warned
// that file presence is done, which means some level of disk I/O.
func ParseDump(r io.Reader, out io.Writer, guesspaths bool) (*Context, error) {
	goroutines, err := parseDump(r, out)
	if len(goroutines) == 0 {
		return nil, err
	}
	c := &Context{
		Goroutines:   goroutines,
		localgoroot:  strings.Replace(runtime.GOROOT(), "\\", "/", -1),
		localgopaths: getGOPATHs(),
	}
	nameArguments(goroutines)
	// Corresponding local values on the host for Context.
	if guesspaths {
		c.findRoots()
		for _, r := range c.Goroutines {
			// Note that this is important to call it even if
			// c.GOROOT == c.localgoroot.
			r.updateLocations(c.GOROOT, c.localgoroot, c.localGomoduleRoot, c.gomodImportPath, c.GOPATHs)
		}
	}
	return c, err
}

// Private stuff.

func parseDump(r io.Reader, out io.Writer) ([]*Goroutine, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(scanLines)
	// Do not enable race detection parsing yet, since it cannot be returned in
	// Context at the moment.
	s := scanningState{}
	for scanner.Scan() {
		line, err := s.scan(scanner.Text())
		if line != "" {
			_, _ = io.WriteString(out, line)
		}
		if err != nil {
			return s.goroutines, err
		}
	}
	return s.goroutines, scanner.Err()
}

// scanLines is similar to bufio.ScanLines except that it:
//     - doesn't drop '\n'
//     - doesn't strip '\r'
//     - returns when the data is bufio.MaxScanTokenSize bytes
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[0 : i+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	if len(data) >= bufio.MaxScanTokenSize {
		// Returns the line even if it is not at EOF nor has a '\n', otherwise the
		// scanner will return bufio.ErrTooLong which is definitely not what we
		// want.
		return len(data), data, nil
	}
	return 0, nil, nil
}

const (
	lockedToThread = "locked to thread"
	elided         = "...additional frames elided..."
	// gotRaceHeader1, normal
	raceHeaderFooter = "=================="
	// gotRaceHeader2
	raceHeader = "WARNING: DATA RACE"
)

// These are effectively constants.
var (
	// gotRoutineHeader
	reRoutineHeader = regexp.MustCompile("^([ \t]*)goroutine (\\d+) \\[([^\\]]+)\\]\\:$")
	reMinutes       = regexp.MustCompile(`^(\d+) minutes$`)

	// gotUnavail
	reUnavail = regexp.MustCompile("^(?:\t| +)goroutine running on other thread; stack unavailable")

	// gotFileFunc, gotRaceOperationFile, gotRaceGoroutineFile
	// See gentraceback() in src/runtime/traceback.go for more information.
	// - Sometimes the source file comes up as "<autogenerated>". It is the
	//   compiler than generated these, not the runtime.
	// - The tab may be replaced with spaces when a user copy-paste it, handle
	//   this transparently.
	// - "runtime.gopanic" is explicitly replaced with "panic" by gentraceback().
	// - The +0x123 byte offset is printed when frame.pc > _func.entry. _func is
	//   generated by the linker.
	// - The +0x123 byte offset is not included with generated code, e.g. unnamed
	//   functions "func·006()" which is generally go func() { ... }()
	//   statements. Since the _func is generated at runtime, it's probably why
	//   _func.entry is not set.
	// - C calls may have fp=0x123 sp=0x123 appended. I think it normally happens
	//   when a signal is not correctly handled. It is printed with m.throwing>0.
	//   These are discarded.
	// - For cgo, the source file may be "??".
	reFile = regexp.MustCompile("^(?:\t| +)(\\?\\?|\\<autogenerated\\>|.+\\.(?:c|go|s))\\:(\\d+)(?:| \\+0x[0-9a-f]+)(?:| fp=0x[0-9a-f]+ sp=0x[0-9a-f]+(?:| pc=0x[0-9a-f]+))$")

	// gotCreated
	// Sadly, it doesn't note the goroutine number so we could cascade them per
	// parenthood.
	reCreated = regexp.MustCompile("^created by (.+)$")

	// gotFunc, gotRaceOperationFunc, gotRaceGoroutineFunc
	reFunc = regexp.MustCompile(`^(.+)\((.*)\)$`)

	// Race:
	// See https://github.com/llvm/llvm-project/blob/master/compiler-rt/lib/tsan/rtl/tsan_report.cpp
	// for the code generating these messages. Please note only the block in
	//   #else  // #if !SANITIZER_GO
	// is used.
	// TODO(maruel): "    [failed to restore the stack]\n\n"
	// TODO(maruel): "Global var %s of size %zu at %p declared at %s:%zu\n"

	// gotRaceOperationHeader
	reRaceOperationHeader = regexp.MustCompile(`^(Read|Write) at (0x[0-9a-f]+) by goroutine (\d+):$`)

	// gotRaceOperationHeader
	reRacePreviousOperationHeader = regexp.MustCompile(`^Previous (read|write) at (0x[0-9a-f]+) by goroutine (\d+):$`)

	// gotRaceGoroutineHeader
	reRaceGoroutine = regexp.MustCompile(`^Goroutine (\d+) \((running|finished)\) created at:$`)

	// TODO(maruel): Use it.
	//reRacePreviousOperationMainHeader = regexp.MustCompile("^Previous (read|write) at (0x[0-9a-f]+) by main goroutine:$")
)

// state is the state of the scan to detect and process a stack trace.
type state int

// Initial state is normal. Other states are when a stack trace is detected.
const (
	// Outside a stack trace.
	// to: gotRoutineHeader, raceHeader1
	normal state = iota

	// Panic stack trace:

	// Signature: ""
	// An empty line between goroutines.
	// from: gotFileCreated, gotFileFunc
	// to: gotRoutineHeader, normal
	betweenRoutine
	// Regexp: reRoutineHeader
	// Signature: "goroutine 1 [running]:"
	// Goroutine header was found.
	// from: normal
	// to: gotUnavail, gotFunc
	gotRoutineHeader
	// Regexp: reFunc
	// Signature: "main.main()"
	// Function call line was found.
	// from: gotRoutineHeader
	// to: gotFileFunc
	gotFunc
	// Regexp: reCreated
	// Signature: "created by main.glob..func4"
	// Goroutine creation line was found.
	// from: gotFileFunc
	// to: gotFileCreated
	gotCreated
	// Regexp: reFile
	// Signature: "\t/foo/bar/baz.go:116 +0x35"
	// File header was found.
	// from: gotFunc
	// to: gotFunc, gotCreated, betweenRoutine, normal
	gotFileFunc
	// Regexp: reFile
	// Signature: "\t/foo/bar/baz.go:116 +0x35"
	// File header was found.
	// from: gotCreated
	// to: betweenRoutine, normal
	gotFileCreated
	// Regexp: reUnavail
	// Signature: "goroutine running on other thread; stack unavailable"
	// State when the goroutine stack is instead is reUnavail.
	// from: gotRoutineHeader
	// to: betweenRoutine, gotCreated
	gotUnavail

	// Race detector:

	// Constant: raceHeaderFooter
	// Signature: "=================="
	// from: normal
	// to: normal, gotRaceHeader2
	gotRaceHeader1
	// Constant: raceHeader
	// Signature: "WARNING: DATA RACE"
	// from: gotRaceHeader1
	// to: normal, gotRaceOperationHeader
	gotRaceHeader2
	// Regexp: reRaceOperationHeader, reRacePreviousOperationHeader
	// Signature: "Read at 0x00c0000e4030 by goroutine 7:"
	// A race operation was found.
	// from: gotRaceHeader2
	// to: normal, gotRaceOperationFunc
	gotRaceOperationHeader
	// Regexp: reFunc
	// Signature: "  main.panicRace.func1()"
	// Function that caused the race.
	// from: gotRaceOperationHeader
	// to: normal, gotRaceOperationFile
	gotRaceOperationFunc
	// Regexp: reFile
	// Signature: "\t/foo/bar/baz.go:116 +0x35"
	// File header that caused the race.
	// from: gotRaceOperationFunc
	// to: normal, betweenRaceOperations, gotRaceOperationFunc
	gotRaceOperationFile
	// Signature: ""
	// Empty line between race operations or just after.
	// from: gotRaceOperationFile
	// to: normal, gotRaceOperationHeader, gotRaceGoroutineHeader
	betweenRaceOperations

	// Regexp: reRaceGoroutine
	// Signature: "Goroutine 7 (running) created at:"
	// Goroutine header.
	// from: betweenRaceOperations, betweenRaceGoroutines
	// to: normal, gotRaceOperationHeader
	gotRaceGoroutineHeader
	// Regexp: reFunc
	// Signature: "  main.panicRace.func1()"
	// Function that caused the race.
	// from: gotRaceGoroutineHeader
	// to: normal, gotRaceGoroutineFile
	gotRaceGoroutineFunc
	// Regexp: reFile
	// Signature: "\t/foo/bar/baz.go:116 +0x35"
	// File header that caused the race.
	// from: gotRaceGoroutineFunc
	// to: normal, betweenRaceGoroutines
	gotRaceGoroutineFile
	// Signature: ""
	// Empty line between race stack traces.
	// from: gotRaceGoroutineFile
	// to: normal, gotRaceGoroutineHeader
	betweenRaceGoroutines
)

// raceOp is one of the detected data race operation as detected by the race
// detector.
type raceOp struct {
	write  bool
	addr   uint64
	id     int
	create Stack
}

// scanningState is the state of the scan to detect and process a stack trace
// and stores the traces found.
type scanningState struct {
	// Determines if race detection is enabled. Currently false since scan()
	// would swallow the race detector output, but the data is not part of
	// Context yet.
	raceDetectionEnabled bool

	// goroutines contains all the goroutines found.
	goroutines []*Goroutine

	state       state
	prefix      string
	races       map[int]*raceOp
	goroutineID int
}

// scan scans one line, updates goroutines and move to the next state.
//
// TODO(maruel): Handle corrupted stack cases:
// - missed stack barrier
// - found next stack barrier at 0x123; expected
// - runtime: unexpected return pc for FUNC_NAME called from 0x123
func (s *scanningState) scan(line string) (string, error) {
	/* This is very useful to debug issues in the state machine.
	defer func() {
		log.Printf("scan(%q) -> %s", line, s.state)
	}()
	//*/
	var cur *Goroutine
	if len(s.goroutines) != 0 {
		cur = s.goroutines[len(s.goroutines)-1]
	}
	trimmed := line
	if strings.HasSuffix(line, "\r\n") {
		trimmed = line[:len(line)-2]
	} else if strings.HasSuffix(line, "\n") {
		trimmed = line[:len(line)-1]
	} else {
		// There's two cases:
		// - It's the end of the stream and it's not terminating with EOL character.
		// - The line is longer than bufio.MaxScanTokenSize
		if s.state == normal {
			return line, nil
		}
		// Let it flow. It's possible the last line was trimmed and we still want to parse it.
	}

	if trimmed != "" && s.prefix != "" {
		// This can only be the case if s.state != normal or the line is empty.
		if !strings.HasPrefix(trimmed, s.prefix) {
			prefix := s.prefix
			s.state = normal
			s.prefix = ""
			return "", fmt.Errorf("inconsistent indentation: %q, expected %q", trimmed, prefix)
		}
		trimmed = trimmed[len(s.prefix):]
	}

	switch s.state {
	case normal:
		// We could look for '^panic:' but this is more risky, there can be a lot
		// of junk between this and the stack dump.
		fallthrough

	case betweenRoutine:
		// Look for a goroutine header.
		if match := reRoutineHeader.FindStringSubmatch(trimmed); match != nil {
			if id, err := strconv.Atoi(match[2]); err == nil {
				// See runtime/traceback.go.
				// "<state>, \d+ minutes, locked to thread"
				items := strings.Split(match[3], ", ")
				sleep := 0
				locked := false
				for i := 1; i < len(items); i++ {
					if items[i] == lockedToThread {
						locked = true
						continue
					}
					// Look for duration, if any.
					if match2 := reMinutes.FindStringSubmatch(items[i]); match2 != nil {
						sleep, _ = strconv.Atoi(match2[1])
					}
				}
				g := &Goroutine{
					Signature: Signature{
						State:    items[0],
						SleepMin: sleep,
						SleepMax: sleep,
						Locked:   locked,
					},
					ID:    id,
					First: len(s.goroutines) == 0,
				}
				// Increase performance by always allocating 4 goroutines minimally.
				if s.goroutines == nil {
					s.goroutines = make([]*Goroutine, 0, 4)
				}
				s.goroutines = append(s.goroutines, g)
				s.state = gotRoutineHeader
				s.prefix = match[1]
				return "", nil
			}
		}
		// Switch to race detection mode.
		if s.raceDetectionEnabled && trimmed == raceHeaderFooter {
			// TODO(maruel): We should buffer it in case the next line is not a
			// WARNING so we can output it back.
			s.state = gotRaceHeader1
			return "", nil
		}
		// Fallthrough.
		s.state = normal
		s.prefix = ""
		return line, nil

	case gotRoutineHeader:
		if reUnavail.MatchString(trimmed) {
			// Generate a fake stack entry.
			cur.Stack.Calls = []Call{{SrcPath: "<unavailable>"}}
			// Next line is expected to be an empty line.
			s.state = gotUnavail
			return "", nil
		}
		c := Call{}
		if found, err := parseFunc(&c, trimmed); found {
			// Increase performance by always allocating 4 calls minimally.
			if cur.Stack.Calls == nil {
				cur.Stack.Calls = make([]Call, 0, 4)
			}
			cur.Stack.Calls = append(cur.Stack.Calls, c)
			s.state = gotFunc
			return "", err
		}
		return "", fmt.Errorf("expected a function after a goroutine header, got: %q", strings.TrimSpace(trimmed))

	case gotFunc:
		// cur.Stack.Calls is guaranteed to have at least one item.
		if found, err := parseFile(&cur.Stack.Calls[len(cur.Stack.Calls)-1], trimmed); err != nil {
			return "", err
		} else if !found {
			return "", fmt.Errorf("expected a file after a function, got: %q", strings.TrimSpace(trimmed))
		}
		s.state = gotFileFunc
		return "", nil

	case gotCreated:
		if found, err := parseFile(&cur.CreatedBy, trimmed); err != nil {
			return "", err
		} else if !found {
			return "", fmt.Errorf("expected a file after a created line, got: %q", trimmed)
		}
		s.state = gotFileCreated
		return "", nil

	case gotFileFunc:
		if match := reCreated.FindStringSubmatch(trimmed); match != nil {
			if err := cur.CreatedBy.Func.Init(match[1]); err != nil {
				return "", err
			}
			s.state = gotCreated
			return "", nil
		}
		if elided == trimmed {
			cur.Stack.Elided = true
			// TODO(maruel): New state.
			return "", nil
		}
		c := Call{}
		if found, err := parseFunc(&c, trimmed); found {
			// Increase performance by always allocating 4 calls minimally.
			if cur.Stack.Calls == nil {
				cur.Stack.Calls = make([]Call, 0, 4)
			}
			cur.Stack.Calls = append(cur.Stack.Calls, c)
			s.state = gotFunc
			return "", err
		}
		if trimmed == "" {
			s.state = betweenRoutine
			return "", nil
		}
		// Back to normal state.
		s.state = normal
		s.prefix = ""
		return line, nil

	case gotFileCreated:
		if trimmed == "" {
			s.state = betweenRoutine
			return "", nil
		}
		s.state = normal
		s.prefix = ""
		return line, nil

	case gotUnavail:
		if trimmed == "" {
			s.state = betweenRoutine
			return "", nil
		}
		if match := reCreated.FindStringSubmatch(trimmed); match != nil {
			if err := cur.CreatedBy.Func.Init(match[1]); err != nil {
				return "", err
			}
			s.state = gotCreated
			return "", nil
		}
		return "", fmt.Errorf("expected empty line after unavailable stack, got: %q", strings.TrimSpace(trimmed))

		// Race detector.

	case gotRaceHeader1:
		if raceHeader == trimmed {
			// TODO(maruel): We should buffer it in case the next line is not a
			// WARNING so we can output it back.
			s.state = gotRaceHeader2
			return "", nil
		}
		s.state = normal
		return line, nil

	case gotRaceHeader2:
		if match := reRaceOperationHeader.FindStringSubmatch(trimmed); match != nil {
			w := match[1] == "Write"
			addr, err := strconv.ParseUint(match[2], 0, 64)
			if err != nil {
				return "", fmt.Errorf("failed to parse address on line: %q", strings.TrimSpace(trimmed))
			}
			id, err := strconv.Atoi(match[3])
			if err != nil {
				return "", fmt.Errorf("failed to parse goroutine id on line: %q", strings.TrimSpace(trimmed))
			}
			if s.races != nil {
				panic("internal failure; expected s.races to be nil")
			}
			if s.goroutines != nil {
				panic("internal failure; expected s.goroutines to be nil")
			}
			s.races = make(map[int]*raceOp, 4)
			s.races[id] = &raceOp{write: w, addr: addr, id: id}
			s.goroutines = append(make([]*Goroutine, 0, 4), &Goroutine{ID: id, First: true})
			s.goroutineID = id
			s.state = gotRaceOperationHeader
			return "", nil
		}
		s.state = normal
		return line, nil

	case gotRaceOperationHeader:
		c := Call{}
		if found, err := parseFunc(&c, strings.TrimLeft(trimmed, "\t ")); found {
			// Increase performance by always allocating 4 calls minimally.
			if cur.Stack.Calls == nil {
				cur.Stack.Calls = make([]Call, 0, 4)
			}
			cur.Stack.Calls = append(cur.Stack.Calls, c)
			s.state = gotRaceOperationFunc
			return "", err
		}
		return "", fmt.Errorf("expected a function after a race operation, got: %q", trimmed)

	case gotRaceOperationFunc:
		if found, err := parseFile(&cur.Stack.Calls[len(cur.Stack.Calls)-1], trimmed); err != nil {
			return "", err
		} else if !found {
			return "", fmt.Errorf("expected a file after a race function, got: %q", trimmed)
		}
		s.state = gotRaceOperationFile
		return "", nil

	case gotRaceOperationFile:
		if trimmed == "" {
			s.state = betweenRaceOperations
			return "", nil
		}
		c := Call{}
		if found, err := parseFunc(&c, strings.TrimLeft(trimmed, "\t ")); found {
			cur.Stack.Calls = append(cur.Stack.Calls, c)
			s.state = gotRaceOperationFunc
			return "", err
		}
		return "", fmt.Errorf("expected an empty line after a race file, got: %q", trimmed)

	case betweenRaceOperations:
		// Look for other previous race data operations.
		if match := reRacePreviousOperationHeader.FindStringSubmatch(trimmed); match != nil {
			w := match[1] == "write"
			addr, err := strconv.ParseUint(match[2], 0, 64)
			if err != nil {
				return "", fmt.Errorf("failed to parse address on line: %q", strings.TrimSpace(trimmed))
			}
			id, err := strconv.Atoi(match[3])
			if err != nil {
				return "", fmt.Errorf("failed to parse goroutine id on line: %q", strings.TrimSpace(trimmed))
			}
			s.goroutineID = id
			s.races[s.goroutineID] = &raceOp{write: w, addr: addr, id: id}
			s.goroutines = append(s.goroutines, &Goroutine{ID: id})
			s.state = gotRaceOperationHeader
			return "", nil
		}
		fallthrough

	case betweenRaceGoroutines:
		if match := reRaceGoroutine.FindStringSubmatch(trimmed); match != nil {
			id, err := strconv.Atoi(match[1])
			if err != nil {
				return "", fmt.Errorf("failed to parse goroutine id on line: %q", strings.TrimSpace(trimmed))
			}
			found := false
			for _, g := range s.goroutines {
				if g.ID == id {
					g.State = match[2]
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("unexpected goroutine ID on line: %q", strings.TrimSpace(trimmed))
			}
			s.goroutineID = id
			s.state = gotRaceGoroutineHeader
			return "", nil
		}
		return "", fmt.Errorf("expected an operator or goroutine, got: %q", trimmed)

		// Race stack traces

	case gotRaceGoroutineFunc:
		c := s.races[s.goroutineID].create.Calls
		if found, err := parseFile(&c[len(c)-1], trimmed); err != nil {
			return "", err
		} else if !found {
			return "", fmt.Errorf("expected a file after a race function, got: %q", trimmed)
		}
		// TODO(maruel): Set s.goroutines[].CreatedBy.
		s.state = gotRaceGoroutineFile
		return "", nil

	case gotRaceGoroutineFile:
		if trimmed == "" {
			s.state = betweenRaceGoroutines
			return "", nil
		}
		if trimmed == raceHeaderFooter {
			// Done.
			s.state = normal
			return "", nil
		}
		fallthrough

	case gotRaceGoroutineHeader:
		c := Call{}
		if found, err := parseFunc(&c, strings.TrimLeft(trimmed, "\t ")); found {
			// TODO(maruel): Set s.goroutines[].CreatedBy.
			s.races[s.goroutineID].create.Calls = append(s.races[s.goroutineID].create.Calls, c)
			s.state = gotRaceGoroutineFunc
			return "", err
		}
		return "", fmt.Errorf("expected a function after a race operation or a race file, got: %q", trimmed)

	default:
		return "", errors.New("internal error")
	}
}

// parseFunc only return an error if also returning a Call.
//
// Uses reFunc.
func parseFunc(c *Call, line string) (bool, error) {
	if match := reFunc.FindStringSubmatch(line); match != nil {
		if err := c.Func.Init(match[1]); err != nil {
			return true, err
		}
		for _, a := range strings.Split(match[2], ", ") {
			if a == "..." {
				c.Args.Elided = true
				continue
			}
			if a == "" {
				// Remaining values were dropped.
				break
			}
			v, err := strconv.ParseUint(a, 0, 64)
			if err != nil {
				return true, fmt.Errorf("failed to parse int on line: %q", strings.TrimSpace(line))
			}
			// Increase performance by always allocating 4 values minimally.
			if c.Args.Values == nil {
				c.Args.Values = make([]Arg, 0, 4)
			}
			// Assume the stack was generated with the same bitness (32 vs 64) than
			// the code processing it.
			c.Args.Values = append(c.Args.Values, Arg{Value: v, IsPtr: v > pointerFloor && v < pointerCeiling})
		}
		return true, nil
	}
	return false, nil
}

// parseFile only return an error if also processing a Call.
//
// Uses reFile.
func parseFile(c *Call, line string) (bool, error) {
	if match := reFile.FindStringSubmatch(line); match != nil {
		num, err := strconv.Atoi(match[2])
		if err != nil {
			return true, fmt.Errorf("failed to parse int on line: %q", strings.TrimSpace(line))
		}
		// TODO(maruel): This returns a string slice inside line. We may want to
		// trim memory further.
		c.init(match[1], num)
		return true, nil
	}
	return false, nil
}

// hasSrcPrefix returns true if any of s is the prefix of p.
func hasSrcPrefix(p string, s map[string]string) bool {
	for prefix := range s {
		if strings.HasPrefix(p, prefix+"/src/") || strings.HasPrefix(p, prefix+"/pkg/mod/") {
			return true
		}
	}
	return false
}

// getFiles returns all the source files deduped and ordered.
func getFiles(goroutines []*Goroutine) []string {
	files := map[string]struct{}{}
	for _, g := range goroutines {
		for _, c := range g.Stack.Calls {
			files[c.SrcPath] = struct{}{}
		}
	}
	if len(files) == 0 {
		return nil
	}
	out := make([]string, 0, len(files))
	for f := range files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// splitPath splits a path using "/" as separator into its components.
//
// The first item has its initial path separator kept.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	var out []string
	s := ""
	for _, c := range p {
		if c != '/' || (len(out) == 0 && strings.Count(s, "/") == len(s)) {
			s += string(c)
		} else if s != "" {
			out = append(out, s)
			s = ""
		}
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// isFile returns true if the path is a valid file.
func isFile(p string) bool {
	// TODO(maruel): Is it faster to open the file or to stat it? Worth a perf
	// test on Windows.
	i, err := os.Stat(p)
	return err == nil && !i.IsDir()
}

// rootedIn returns a root if the file split in parts is rooted in root.
//
// Uses "/" as path separator.
func rootedIn(root string, parts []string) string {
	//log.Printf("rootIn(%s, %v)", root, parts)
	for i := 1; i < len(parts); i++ {
		suffix := pathJoin(parts[i:]...)
		if isFile(pathJoin(root, suffix)) {
			return pathJoin(parts[:i]...)
		}
	}
	return ""
}

// reModule find the module line in a go.mod file. It works even on CRLF file.
var reModule = regexp.MustCompile(`(?m)^module\s+([^\n\r]+)\r?$`)

// isGoModule returns the string to the directory containing a go.mod/go.sum
// files pair, and the go import path it represents, if found.
func isGoModule(parts []string) (string, string) {
	for i := len(parts); i > 0; i-- {
		prefix := pathJoin(parts[:i]...)
		if isFile(pathJoin(prefix, "go.sum")) {
			b, err := ioutil.ReadFile(pathJoin(prefix, "go.mod"))
			if err != nil {
				continue
			}
			if match := reModule.FindSubmatch(b); match != nil {
				return prefix, string(match[1])
			}
		}
	}
	return "", ""
}

// findRoots sets member GOROOT, GOPATHs and localGomoduleRoot.
//
// This causes disk I/O as it checks for file presence.
//
// Returns the number of missing files.
func (c *Context) findRoots() int {
	c.GOPATHs = map[string]string{}
	missing := 0
	for _, f := range getFiles(c.Goroutines) {
		// TODO(maruel): Could a stack dump have mixed cases? I think it's
		// possible, need to confirm and handle.
		//log.Printf("  Analyzing %s", f)

		// First checks skip file I/O.
		if c.GOROOT != "" && strings.HasPrefix(f, c.GOROOT+"/src/") {
			// stdlib.
			continue
		}
		if hasSrcPrefix(f, c.GOPATHs) {
			// $GOPATH/src or go.mod dependency in $GOPATH/pkg/mod.
			continue
		}

		// At this point, disk will be looked up.
		parts := splitPath(f)
		if c.GOROOT == "" {
			if r := rootedIn(c.localgoroot+"/src", parts); r != "" {
				c.GOROOT = r[:len(r)-4]
				//log.Printf("Found GOROOT=%s", c.GOROOT)
				continue
			}
		}
		found := false
		for _, l := range c.localgopaths {
			if r := rootedIn(l+"/src", parts); r != "" {
				//log.Printf("Found GOPATH=%s", r[:len(r)-4])
				c.GOPATHs[r[:len(r)-4]] = l
				found = true
				break
			}
			if r := rootedIn(l+"/pkg/mod", parts); r != "" {
				//log.Printf("Found GOPATH=%s", r[:len(r)-8])
				c.GOPATHs[r[:len(r)-8]] = l
				found = true
				break
			}
		}
		// If the source is not found, it's probably a go module.
		if !found {
			if c.localGomoduleRoot == "" && len(parts) > 1 {
				// Search upward looking for a go.mod/go.sum pair.
				c.localGomoduleRoot, c.gomodImportPath = isGoModule(parts[:len(parts)-1])
			}
			if c.localGomoduleRoot != "" && strings.HasPrefix(f, c.localGomoduleRoot+"/") {
				continue
			}
		}
		if !found {
			// If the source is not found, just too bad.
			//log.Printf("Failed to find locally: %s", f)
			missing++
		}
	}
	return missing
}

// getGOPATHs returns parsed GOPATH or its default, using "/" as path separator.
func getGOPATHs() []string {
	var out []string
	if gp := os.Getenv("GOPATH"); gp != "" {
		for _, v := range filepath.SplitList(gp) {
			// Disallow non-absolute paths?
			if v != "" {
				v = strings.Replace(v, "\\", "/", -1)
				// Trim trailing "/".
				if l := len(v); v[l-1] == '/' {
					v = v[:l-1]
				}
				out = append(out, v)
			}
		}
	}
	if len(out) == 0 {
		homeDir := ""
		u, err := user.Current()
		if err != nil {
			homeDir = os.Getenv("HOME")
			if homeDir == "" {
				panic(fmt.Sprintf("Could not get current user or $HOME: %s\n", err.Error()))
			}
		} else {
			homeDir = u.HomeDir
		}
		out = []string{strings.Replace(homeDir+"/go", "\\", "/", -1)}
	}
	return out
}
