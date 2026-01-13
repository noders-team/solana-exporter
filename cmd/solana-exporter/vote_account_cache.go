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
