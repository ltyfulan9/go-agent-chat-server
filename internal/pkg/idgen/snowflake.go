package idgen

import (
	"strconv"
	"sync"
	"time"
)

const (
	customEpoch  int64 = 1704067200000 // 2024-01-01 00:00:00 UTC in milliseconds
	nodeBits           = 10
	sequenceBits       = 12

	maxNodeID   = -1 ^ (-1 << nodeBits)
	maxSequence = -1 ^ (-1 << sequenceBits)

	nodeShift      = sequenceBits
	timestampShift = nodeBits + sequenceBits
)

type Snowflake struct {
	mu       sync.Mutex
	nodeID   int64
	lastMs   int64
	sequence int64
}

func NewSnowflake(nodeID int64) *Snowflake {
	if nodeID < 0 || nodeID > maxNodeID {
		nodeID = 1
	}

	return &Snowflake{
		nodeID: nodeID,
		lastMs: -1,
	}
}

func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := currentMs()

	if now < s.lastMs {
		now = s.waitNextMs(s.lastMs)
	}

	if now == s.lastMs {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			now = s.waitNextMs(s.lastMs)
		}
	} else {
		s.sequence = 0
	}

	s.lastMs = now

	return ((now - customEpoch) << timestampShift) | (s.nodeID << nodeShift) | s.sequence
}

func (s *Snowflake) NextString() string {
	return strconv.FormatInt(s.NextID(), 10)
}

func (s *Snowflake) waitNextMs(lastMs int64) int64 {
	now := currentMs()
	for now <= lastMs {
		time.Sleep(time.Millisecond)
		now = currentMs()
	}
	return now
}

func currentMs() int64 {
	return time.Now().UnixMilli()
}

var defaultGenerator = NewSnowflake(1)

func NewID() string {
	return defaultGenerator.NextString()
}
