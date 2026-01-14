package main

import (
	"context"
	"sync"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"go.uber.org/zap"
)

const (
	// VoteAccountCacheTTL is the time-to-live for cached vote account data
	VoteAccountCacheTTL = 10 * time.Second
)

type (
	// VoteAccountCacheEntry represents a cached vote account data entry
	VoteAccountCacheEntry struct {
		Data      rpc.VoteAccountData
		Timestamp time.Time
	}

	// VoteAccountCache provides caching for vote account data to reduce RPC calls
	VoteAccountCache struct {
		mu       sync.RWMutex
		cache    map[string]*VoteAccountCacheEntry
		rpcClient *rpc.Client
		logger   *zap.SugaredLogger
	}
)

// NewVoteAccountCache creates a new vote account cache
func NewVoteAccountCache(rpcClient *rpc.Client) *VoteAccountCache {
	return &VoteAccountCache{
		cache:    make(map[string]*VoteAccountCacheEntry),
		rpcClient: rpcClient,
		logger:   slog.Get(),
	}
}

// Get retrieves vote account data from cache or fetches it if not cached or expired
// Also fetches latency data from raw vote state if available
func (c *VoteAccountCache) Get(ctx context.Context, votekey string) (*rpc.VoteAccountData, error) {
	// Check cache first
	c.mu.RLock()
	entry, exists := c.cache[votekey]
	c.mu.RUnlock()

	if exists && time.Since(entry.Timestamp) < VoteAccountCacheTTL {
		c.logger.Debugf("Vote account cache hit for %s", votekey)
		return &entry.Data, nil
	}

	// Cache miss or expired - fetch from RPC
	c.logger.Debugf("Vote account cache miss for %s, fetching from RPC", votekey)
	var voteAccountData rpc.VoteAccountData
	_, err := rpc.GetAccountInfo(ctx, c.rpcClient, rpc.CommitmentFinalized, votekey, &voteAccountData)
	if err != nil {
		return nil, err
	}

	// Try to get latency data from raw vote state
	c.logger.Infof("Attempting to fetch raw vote account data for latency extraction (votekey: %s)", votekey)
	rawData, err := rpc.GetAccountInfoRaw(ctx, c.rpcClient, rpc.CommitmentFinalized, votekey)
	if err == nil && rawData != "" {
		c.logger.Infof("Got raw vote account data, length: %d chars, attempting deserialization", len(rawData))
		latencies, err := rpc.DeserializeVoteStateLatencies(rawData)
		if err == nil && len(latencies) > 0 {
			// Map latencies to votes by slot
			latencyMap := make(map[int64]uint8)
			for _, lat := range latencies {
				latencyMap[int64(lat.Slot)] = lat.Latency
			}

			// Update votes with latency information
			latencyCount := 0
			for i := range voteAccountData.Votes {
				if latency, ok := latencyMap[voteAccountData.Votes[i].Slot]; ok {
					voteAccountData.Votes[i].Latency = &latency
					latencyCount++
				}
			}

			c.logger.Infof("Successfully extracted latency data: %d latencies from vote state, matched %d/%d votes", 
				len(latencies), latencyCount, len(voteAccountData.Votes))
		} else {
			if err != nil {
				c.logger.Warnf("Failed to deserialize vote state latencies: %v (using estimated latency)", err)
			} else {
				c.logger.Warnf("No latencies extracted from vote state (got 0 latencies) (using estimated latency)")
			}
		}
	} else {
		if err != nil {
			c.logger.Warnf("Failed to get raw vote account data: %v (using estimated latency)", err)
		} else {
			c.logger.Warnf("Raw vote account data is empty (using estimated latency)")
		}
	}

	// Update cache
	c.mu.Lock()
	c.cache[votekey] = &VoteAccountCacheEntry{
		Data:      voteAccountData,
		Timestamp: time.Now(),
	}
	c.mu.Unlock()

	return &voteAccountData, nil
}

// Invalidate removes an entry from the cache
func (c *VoteAccountCache) Invalidate(votekey string) {
	c.mu.Lock()
	delete(c.cache, votekey)
	c.mu.Unlock()
	c.logger.Debugf("Invalidated vote account cache for %s", votekey)
}

// Clear removes all entries from the cache
func (c *VoteAccountCache) Clear() {
	c.mu.Lock()
	c.cache = make(map[string]*VoteAccountCacheEntry)
	c.mu.Unlock()
	c.logger.Debug("Cleared vote account cache")
}

// CleanupExpired removes expired entries from the cache
func (c *VoteAccountCache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for votekey, entry := range c.cache {
		if now.Sub(entry.Timestamp) >= VoteAccountCacheTTL {
			delete(c.cache, votekey)
		}
	}
}
