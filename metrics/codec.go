package metrics

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EncodeMember renders a Point as a sorted-set member string of the form
// "<unixMillis>:<value>". The millisecond timestamp prefix doubles as the
// sorted-set score, and including it in the member keeps members unique even
// when two observations share a value. The value is formatted with -1
// precision so it round-trips losslessly through ParseMember.
func EncodeMember(p Point) string {
	return strconv.FormatInt(p.At.UnixMilli(), 10) + ":" +
		strconv.FormatFloat(p.Value, 'g', -1, 64)
}

// ParseMember reverses EncodeMember. It returns an error for any member that
// does not match the "<unixMillis>:<value>" shape so a corrupt entry surfaces
// loudly rather than decoding to a zero Point.
func ParseMember(member string) (Point, error) {
	tsPart, valPart, ok := strings.Cut(member, ":")
	if !ok {
		return Point{}, fmt.Errorf("metrics: malformed member %q: missing ':' separator", member)
	}
	ms, err := strconv.ParseInt(tsPart, 10, 64)
	if err != nil {
		return Point{}, fmt.Errorf("metrics: malformed member %q: bad timestamp: %w", member, err)
	}
	val, err := strconv.ParseFloat(valPart, 64)
	if err != nil {
		return Point{}, fmt.Errorf("metrics: malformed member %q: bad value: %w", member, err)
	}
	// Normalise to UTC: the wire format is a location-agnostic epoch, so the
	// canonical decoded instant is UTC. This keeps decoded points comparable
	// regardless of the process's local zone.
	return Point{At: time.UnixMilli(ms).UTC(), Value: val}, nil
}
