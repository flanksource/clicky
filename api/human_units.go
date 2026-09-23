// Byte and duration humanization copied from github.com/flanksource/commons v1.59.0
// so package api does not depend on the commons text package:
//
//   - humanizeBytes: text/bytes.go HumanizeBytes, narrowed to the int64 input
//     api passes. Apache License 2.0; Copyright (c) 2015-Present
//     CloudFoundry.org Foundation, Inc. and (c) 2013-2015 Pivotal Software, Inc.
//     (http://www.apache.org/licenses/LICENSE-2.0).
//   - humanizeDuration: text/duration.go HumanizeDuration, i.e. duration/duration.go
//     Duration.String, narrowed to the >= 24h durations api passes. That file is
//     forked from https://git.maze.io/go/duration and carries this notice:
//
// MIT License
// Copyright Wijnand Modderman-Lenstra (maze.io)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	unitByte = 1 << (10 * iota)
	unitKilobyte
	unitMegabyte
	unitGigabyte
	unitTerabyte
	unitPetabyte
	unitExabyte
)

// humanizeBytes returns a byte count of the form 10M, 12.5K, choosing the unit
// that yields the smallest number >= 1. Negative sizes wrap to uint64, as in commons.
func humanizeBytes(size int64) string {
	unit := ""
	bytes := uint64(size)
	value := float64(bytes)

	switch {
	case bytes >= unitExabyte:
		unit = "E"
		value = value / unitExabyte
	case bytes >= unitPetabyte:
		unit = "P"
		value = value / unitPetabyte
	case bytes >= unitTerabyte:
		unit = "T"
		value = value / unitTerabyte
	case bytes >= unitGigabyte:
		unit = "G"
		value = value / unitGigabyte
	case bytes >= unitMegabyte:
		unit = "M"
		value = value / unitMegabyte
	case bytes >= unitKilobyte:
		unit = "K"
		value = value / unitKilobyte
	case bytes >= unitByte:
		unit = "B"
	case bytes == 0:
		return "0B"
	}

	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

const (
	durationDay  = 24 * time.Hour
	durationWeek = 7 * durationDay
)

// humanizeDuration formats a duration of at least one day as e.g. "3d2h6m" or
// "4w3d2h": over a day it is truncated to minutes, over a week to hours.
func humanizeDuration(d time.Duration) string {
	if d < durationDay {
		panic(fmt.Sprintf("humanizeDuration: %s is shorter than a day", d))
	}
	if d.Hours() > 24*7 {
		d = d - d%time.Hour
	} else if d.Hours() > 24 {
		d = d - d%time.Minute
	}
	var buf [32]byte
	var w int
	u := uint64(d)
	switch {
	case u > uint64(durationWeek):
		w = formatWeeksDaysHours(buf[:], u)
	case u > uint64(durationDay):
		w = formatDaysHoursMinutes(buf[:], u)
	default:
		w = formatHoursMinutesSeconds(buf[:], u)
	}
	return stripZeroDurationUnits(string(buf[w:]))
}

func formatWeeksDaysHours(buf []byte, u uint64) int {
	w := len(buf)
	w--
	buf[w] = 'h'
	u /= uint64(time.Hour)
	w = formatUintTail(buf[:w], u%24)
	u /= 24
	if u > 0 {
		w--
		buf[w] = 'd'
		w = formatUintTail(buf[:w], u%7)
		u /= 7
		if u > 0 {
			w--
			buf[w] = 'w'
			w = formatUintTail(buf[:w], u)
		}
	}
	return w
}

func formatDaysHoursMinutes(buf []byte, u uint64) int {
	w := len(buf)
	w--
	buf[w] = 'm'
	u /= uint64(time.Minute)
	w = formatUintTail(buf[:w], u%60)
	u /= 60
	if u > 0 {
		w--
		buf[w] = 'h'
		w = formatUintTail(buf[:w], u%24)
		u /= 24
		if u > 0 {
			w--
			buf[w] = 'd'
			w = formatUintTail(buf[:w], u)
		}
	}
	return w
}

// formatHoursMinutesSeconds is only reached for exactly one day, which has no
// sub-second fraction, so the commons fraction formatting is not needed.
func formatHoursMinutesSeconds(buf []byte, u uint64) int {
	w := len(buf)
	w--
	buf[w] = 's'
	u /= uint64(time.Second)
	w = formatUintTail(buf[:w], u%60)
	u /= 60
	if u > 0 {
		w--
		buf[w] = 'm'
		w = formatUintTail(buf[:w], u%60)
		u /= 60
		if u > 0 {
			w--
			buf[w] = 'h'
			w = formatUintTail(buf[:w], u)
		}
	}
	return w
}

// stripZeroDurationUnits drops a standalone "0m" and a trailing standalone "0s",
// never the zero digit of a multi-digit value such as "30m" or "30s".
func stripZeroDurationUnits(result string) string {
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	if i := strings.Index(result, "0m"); i >= 0 {
		if (i+2 >= len(result) || result[i+2] != 's') && (i == 0 || !isDigit(result[i-1])) {
			result = result[:i] + result[i+2:]
		}
	}
	if strings.HasSuffix(result, "0s") && len(result) > 2 && !isDigit(result[len(result)-3]) {
		result = result[:len(result)-2]
	}
	return result
}

// formatUintTail formats v into the tail of buf and returns where the output begins.
func formatUintTail(buf []byte, v uint64) int {
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	return w
}
