package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	BatchLabel       = "batch"
	SlotRangeLabel   = "slot_range"
	LatencyLabel     = "avg_latency"
	MissedTVCsLabel  = "missed_tvcs"
	MissedSlotsLabel = "missed_slots"
	StartTimeLabel   = "start_time"

	// Vote batch analysis parameters
	MaxVoteBatchGap   = 50    // Maximum slots between votes in same batch
	MinBatchSize      = 100   // Minimum slots to consider as a batch
	MaxLatencySeconds = 30    // Maximum expected voting latency
	FixedBatchSize    = 20000 // Fixed batch size in slots (each batch = 20000 slots)

	// Timely Vote Credits (TVC) parameters (from SIMD-0033)
	MaxTVCPerVote  = 10 // Maximum credits for latency 1-3 (grace period)
	TVCGracePeriod = 3  // Grace period slots (latency 1-3 = max credits)
	MinTVCPerVote  = 1  // Minimum credits (never 0)
)

type (
	// VoteBatch represents a group of consecutive votes
	VoteBatch struct {
		ID            int       `json:"batch_id"`
		StartTime     time.Time `json:"start_time"`
		StartSlot     int64     `json:"start_slot"`
		EndSlot       int64     `json:"end_slot"`
		SlotRange     string    `json:"slot_range"`
		AvgLatency    float64   `json:"avg_latency"`
		TotalSlots    int64     `json:"total_slots"`
		VotedSlots    int64     `json:"voted_slots"`
		VoteCount     int64     `json:"vote_count"`
		EarnedCredits int64     `json:"earned_credits"` // Credits earned based on vote latencies
		Votes         []Vote    `json:"votes"`
		// Deprecated fields (kept for backward compatibility, always 0)
		MissedTVCs  int64   `json:"missed_tvcs"`
		MissedSlots int64   `json:"missed_slots"`
		Performance float64 `json:"performance_pct"`
	}

	// VoteBatchAnalyzer analyzes vote patterns and detects gaps
	VoteBatchAnalyzer struct {
		rpcClient        *rpc.Client
		logger           *zap.SugaredLogger
		config           *ExporterConfig
		voteAccountCache *VoteAccountCache

		// Metrics for vote batch analysis
		VoteBatchMissedTVCs  *prometheus.GaugeVec
		VoteBatchMissedSlots *prometheus.GaugeVec
		VoteBatchAvgLatency  *prometheus.GaugeVec
		VoteBatchPerformance *prometheus.GaugeVec
		VoteBatchSlotRange   *prometheus.GaugeVec
		VoteBatchCount       *prometheus.GaugeVec
	}

	// VoteGap represents a gap in voting pattern
	VoteGap struct {
		StartSlot  int64     `json:"start_slot"`
		EndSlot    int64     `json:"end_slot"`
		Duration   int64     `json:"duration_slots"`
		Timestamp  time.Time `json:"timestamp"`
		MissedTVCs int64     `json:"missed_tvcs"`
	}

	// Vote represents a validator vote (local copy from rpc package)
	Vote struct {
		ConfirmationCount int64  `json:"confirmationCount"`
		Slot              int64  `json:"slot"`
		Latency           *uint8 `json:"latency,omitempty"` // Actual latency from vote state (if available)
	}
)

func NewVoteBatchAnalyzer(rpcClient *rpc.Client, config *ExporterConfig, voteAccountCache *VoteAccountCache) *VoteBatchAnalyzer {
	logger := slog.Get()

	return &VoteBatchAnalyzer{
		rpcClient:        rpcClient,
		logger:           logger,
		config:           config,
		voteAccountCache: voteAccountCache,

		VoteBatchMissedTVCs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_missed_tvcs",
				Help: fmt.Sprintf("Number of missed TVCs per vote batch (grouped by %s, %s, %s)", NodekeyLabel, VotekeyLabel, BatchLabel),
			},
			[]string{NodekeyLabel, VotekeyLabel, BatchLabel, SlotRangeLabel},
		),

		VoteBatchMissedSlots: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_missed_slots",
				Help: fmt.Sprintf("Number of missed slots per vote batch (grouped by %s, %s, %s)", NodekeyLabel, VotekeyLabel, BatchLabel),
			},
			[]string{NodekeyLabel, VotekeyLabel, BatchLabel, SlotRangeLabel},
		),

		VoteBatchAvgLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_avg_latency_seconds",
				Help: fmt.Sprintf("Average voting latency per batch in seconds (grouped by %s, %s, %s)", NodekeyLabel, VotekeyLabel, BatchLabel),
			},
			[]string{NodekeyLabel, VotekeyLabel, BatchLabel, SlotRangeLabel},
		),

		VoteBatchPerformance: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_performance_pct",
				Help: fmt.Sprintf("Vote performance percentage per batch (grouped by %s, %s, %s)", NodekeyLabel, VotekeyLabel, BatchLabel),
			},
			[]string{NodekeyLabel, VotekeyLabel, BatchLabel, SlotRangeLabel},
		),

		VoteBatchSlotRange: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_slot_range",
				Help: fmt.Sprintf("Slot range covered by vote batch (grouped by %s, %s, %s)", NodekeyLabel, VotekeyLabel, BatchLabel),
			},
			[]string{NodekeyLabel, VotekeyLabel, BatchLabel, SlotRangeLabel},
		),

		VoteBatchCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_count_total",
				Help: fmt.Sprintf("Total number of vote batches analyzed (grouped by %s, %s)", NodekeyLabel, VotekeyLabel),
			},
			[]string{NodekeyLabel, VotekeyLabel},
		),
	}
}

// AnalyzeVoteBatches analyzes vote account data and extracts batch information
func (v *VoteBatchAnalyzer) AnalyzeVoteBatches(
	ctx context.Context,
	voteAccountData *rpc.VoteAccountData,
	nodekey, votekey string,
) ([]VoteBatch, error) {
	if len(voteAccountData.Votes) == 0 {
		return nil, fmt.Errorf("no votes found in vote account data")
	}

	// Sort votes by slot (should already be sorted, but ensure it)
	votes := make([]Vote, len(voteAccountData.Votes))
	for i, v := range voteAccountData.Votes {
		votes[i] = Vote{
			ConfirmationCount: v.ConfirmationCount,
			Slot:              v.Slot,
			Latency:           v.Latency, // Include latency if available from vote state
		}
	}
	sort.Slice(votes, func(i, j int) bool {
		return votes[i].Slot < votes[j].Slot
	})

	// Get current slot for latency calculation
	currentSlot, err := v.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		v.logger.Warnf("Failed to get current slot for latency calculation: %v", err)
		currentSlot = votes[len(votes)-1].Slot + 10 // Fallback estimate
	}

	// Group votes into fixed-size batches (20000 slots each)
	v.logger.Infof("Analyzing votes for validator %s: total votes=%d, first slot=%d, last slot=%d, current slot=%d",
		nodekey, len(votes), votes[0].Slot, votes[len(votes)-1].Slot, currentSlot)

	batches := v.groupVotesIntoFixedBatches(votes, currentSlot)
	v.logger.Infof("Analyzed %d vote batches for validator %s", len(batches), nodekey)

	return batches, nil
}

// groupVotesIntoFixedBatches groups votes into fixed-size batches of 20000 slots each
func (v *VoteBatchAnalyzer) groupVotesIntoFixedBatches(votes []Vote, currentSlot int64) []VoteBatch {
	if len(votes) == 0 {
		return nil
	}

	// Calculate batch numbers (1-based)
	// Batch 1: slots 0-19999, Batch 2: 20000-39999, etc.
	firstSlot := votes[0].Slot
	firstBatchNum := int(firstSlot/FixedBatchSize) + 1
	currentBatchNum := int(currentSlot/FixedBatchSize) + 1

	var batches []VoteBatch
	voteIndex := 0

	// Process each fixed batch interval from first to current
	for batchNum := firstBatchNum; batchNum <= currentBatchNum; batchNum++ {
		// Calculate batch boundaries (0-based slots)
		batchStartSlot := int64(batchNum-1) * FixedBatchSize
		batchEndSlot := int64(batchNum)*FixedBatchSize - 1

		// Collect all votes in this batch interval
		var batchVotes []Vote
		skippedBefore := 0
		for voteIndex < len(votes) {
			voteSlot := votes[voteIndex].Slot
			if voteSlot < batchStartSlot {
				// Skip votes before this batch
				skippedBefore++
				voteIndex++
				continue
			}
			if voteSlot > batchEndSlot {
				// This vote belongs to next batch
				break
			}
			batchVotes = append(batchVotes, votes[voteIndex])
			voteIndex++
		}

		// Log detailed info for current/live batch
		if batchNum == currentBatchNum {
			v.logger.Infof("LIVE BATCH %d: slots %d-%d (current=%d), votes collected=%d, skipped before=%d, voteIndex=%d/%d",
				batchNum, batchStartSlot, batchEndSlot, currentSlot, len(batchVotes), skippedBefore, voteIndex, len(votes))
		}

		// Create batch (even if no votes, to show missed slots)
		batch := VoteBatch{
			ID:        batchNum,
			StartSlot: batchStartSlot,
			EndSlot:   batchEndSlot,
			Votes:     batchVotes,
		}

		// Calculate batch start time
		batch.StartTime = v.estimateSlotTime(batchStartSlot, currentSlot)

		// Finalize batch metrics
		batch = v.finalizeFixedBatch(batch, currentSlot)

		// Log batch info for debugging (detailed for live batch)
		if batchNum == currentBatchNum {
			activeRangeInfo := ""
			if len(batchVotes) > 0 {
				activeStart := batchVotes[0].Slot
				activeEnd := batchVotes[len(batchVotes)-1].Slot
				isLive := batch.EndSlot >= currentSlot
				if isLive {
					activeEnd = currentSlot
				}
				activeRangeInfo = fmt.Sprintf(", activeRange=%d-%d", activeStart, activeEnd)
			}
			v.logger.Infof("BATCH %d FINAL: batchSlots=%d-%d%s, totalVotes=%d, uniqueVotedSlots=%d, missedSlots=%d, activeRangeSlots=%d, performance=%.2f%%",
				batchNum, batch.StartSlot, batch.EndSlot, activeRangeInfo, len(batchVotes), batch.VotedSlots, batch.MissedSlots, batch.TotalSlots, batch.Performance)
			// Log first and last few votes in batch
			if len(batchVotes) > 0 {
				firstFew := len(batchVotes)
				if firstFew > 5 {
					firstFew = 5
				}
				v.logger.Infof("  First %d votes: %v", firstFew, batchVotes[:firstFew])
				if len(batchVotes) > 5 {
					lastFew := len(batchVotes) - 5
					if lastFew < 0 {
						lastFew = 0
					}
					v.logger.Infof("  Last %d votes: %v", len(batchVotes)-lastFew, batchVotes[lastFew:])
				}
			}
		} else {
			v.logger.Debugf("Batch %d: slots %d-%d, votes=%d, votedSlots=%d, missedSlots=%d, totalSlots=%d",
				batchNum, batchStartSlot, batchEndSlot, len(batchVotes), batch.VotedSlots, batch.MissedSlots, batch.TotalSlots)
		}

		batches = append(batches, batch)
	}

	return batches
}

// finalizeFixedBatch calculates metrics for a fixed-size batch
func (v *VoteBatchAnalyzer) finalizeFixedBatch(batch VoteBatch, currentSlot int64) VoteBatch {
	// For current/live batch (not yet completed), adjust end slot to current slot
	isLiveBatch := batch.EndSlot >= currentSlot
	if isLiveBatch {
		batch.EndSlot = currentSlot
		batch.TotalSlots = currentSlot - batch.StartSlot + 1
	} else {
		// For completed batches, use full 20000 slots
		batch.TotalSlots = FixedBatchSize
	}

	// Count voted slots (unique slots voted on)
	// In Solana, each vote covers one slot, and validator gets 1 credit per voted slot
	votedSlotMap := make(map[int64]bool)
	for _, vote := range batch.Votes {
		// Only count votes within batch boundaries
		if vote.Slot >= batch.StartSlot && vote.Slot <= batch.EndSlot {
			votedSlotMap[vote.Slot] = true
		}
	}
	batch.VotedSlots = int64(len(votedSlotMap))

	// Calculate TVCs using Timely Vote Credits (TVC) logic from SIMD-0033
	// IMPORTANT: TVC depends on vote latency!
	// - Latency 1-3: 10 credits (grace period)
	// - Latency 4: 9 credits, Latency 5: 8 credits, etc.
	// - Minimum: 1 credit (never 0)
	//
	// CRITICAL: Validators DON'T vote for every slot - they only vote for blocks they confirm!
	// We CANNOT calculate "missed" TVCs because we don't know how many blocks were in the batch.
	// Real TVC are already calculated by protocol and stored in epochCredits from getVoteAccounts.
	//
	// For batches, we show only REAL metrics:
	// - Vote count (how many votes in this batch)
	// - Average latency (from actual vote state)
	// - Earned credits (calculated from latencies for display)
	// - Performance is based on vote frequency vs typical (not "missed slots")

	var totalEarnedCredits int64
	var totalLatencySum int64
	var latencyCount int64

	// Calculate earned credits based on vote latencies
	// Use actual latency from vote state if available, otherwise estimate from ConfirmationCount
	for _, vote := range batch.Votes {
		if vote.Slot >= batch.StartSlot && vote.Slot <= batch.EndSlot {
			var latency int64

			// Use actual latency from vote state if available (from bincode deserialization)
			if vote.Latency != nil {
				latency = int64(*vote.Latency)
			} else {
				// Fallback: estimate latency based on ConfirmationCount
				// ConfirmationCount = 0: vote sent early, assume latency 1-3 (max 10 credits)
				latency = int64(1) // Default to grace period (max credits)
				if vote.ConfirmationCount > 0 {
					latency = vote.ConfirmationCount + 1
				}
			}

			// Calculate credits based on latency (TVC formula from SIMD-0033)
			credits := v.calculateTVCForLatency(latency)
			totalEarnedCredits += credits
			totalLatencySum += latency
			latencyCount++
		}
	}

	// Calculate average latency for display (convert to seconds: ~400ms per slot)
	if latencyCount > 0 {
		batch.AvgLatency = float64(totalLatencySum) / float64(latencyCount) * 0.4
	}

	// CRITICAL: We cannot calculate "missed" TVCs for batches because:
	// 1. Validators only vote for blocks they confirm, not every slot
	// 2. We don't know how many blocks were produced in this batch
	// 3. Real TVC are already in epochCredits from RPC
	//
	// Instead, we show vote frequency metrics:
	// - How many votes in this batch (actualVotes)
	// - Typical vote frequency is ~every 32 slots (lockout period)
	// - Performance = actual votes / expected votes (based on frequency)

	// Estimate expected votes based on typical vote frequency
	// In Solana, validators typically vote every ~32 slots due to lockout
	expectedVoteFrequency := int64(32)
	expectedVotes := batch.TotalSlots / expectedVoteFrequency
	if expectedVotes < 1 {
		expectedVotes = 1
	}

	actualVotes := int64(len(batch.Votes))
	batch.VoteCount = actualVotes
	batch.EarnedCredits = totalEarnedCredits

	// Deprecated: Missed TVCs and Missed Slots don't make sense for batches
	// Validators only vote for blocks they confirm, not every slot
	// Real TVC are in epochCredits from RPC, not in batch-level calculations
	batch.MissedTVCs = 0
	batch.MissedSlots = 0

	// Deprecated: Performance based on vote frequency is misleading
	// We show real metrics instead: vote count, earned credits, avg latency
	batch.Performance = 0.0

	// Calculate average latency (already calculated above, but ensure it's set)
	if latencyCount == 0 {
		batch.AvgLatency = v.estimateAverageLatency(batch.Votes, currentSlot)
	}

	// Format slot range
	batch.SlotRange = fmt.Sprintf("%d-%d", batch.StartSlot, batch.EndSlot)

	return batch
}

// groupVotesIntoBatches groups consecutive votes into logical batches (legacy method, kept for compatibility)
func (v *VoteBatchAnalyzer) groupVotesIntoBatches(votes []Vote, currentSlot int64) []VoteBatch {
	if len(votes) == 0 {
		return nil
	}

	var batches []VoteBatch
	currentBatch := VoteBatch{
		ID:        1,
		StartSlot: votes[0].Slot,
		Votes:     []Vote{votes[0]},
	}

	for i := 1; i < len(votes); i++ {
		vote := votes[i]
		prevVote := votes[i-1]
		gap := vote.Slot - prevVote.Slot

		// If gap is too large, start new batch
		if gap > MaxVoteBatchGap {
			// Finalize current batch
			currentBatch = v.finalizeBatch(currentBatch, currentSlot, len(batches)+1)
			batches = append(batches, currentBatch)

			// Start new batch
			currentBatch = VoteBatch{
				ID:        len(batches) + 1,
				StartSlot: vote.Slot,
				Votes:     []Vote{vote},
			}
		} else {
			// Add to current batch
			currentBatch.Votes = append(currentBatch.Votes, vote)
		}
	}

	// Finalize last batch
	if len(currentBatch.Votes) > 0 {
		currentBatch = v.finalizeBatch(currentBatch, currentSlot, len(batches)+1)
		batches = append(batches, currentBatch)
	}

	return batches
}

// finalizeBatch calculates all batch metrics
func (v *VoteBatchAnalyzer) finalizeBatch(batch VoteBatch, currentSlot int64, batchID int) VoteBatch {
	if len(batch.Votes) == 0 {
		return batch
	}

	batch.ID = batchID
	batch.EndSlot = batch.Votes[len(batch.Votes)-1].Slot
	batch.TotalSlots = batch.EndSlot - batch.StartSlot + 1
	batch.VotedSlots = int64(len(batch.Votes))
	batch.MissedSlots = batch.TotalSlots - batch.VotedSlots
	batch.SlotRange = fmt.Sprintf("%d-%d", batch.StartSlot, batch.EndSlot)

	// Calculate performance percentage
	if batch.TotalSlots > 0 {
		batch.Performance = float64(batch.VotedSlots) / float64(batch.TotalSlots) * 100.0
	}

	// Estimate missed TVCs (approximate)
	batch.MissedTVCs = batch.MissedSlots

	// Calculate average latency (simplified - based on slot timing)
	// In real implementation, you'd need actual timestamps
	batch.AvgLatency = v.estimateAverageLatency(batch.Votes, currentSlot)

	// Set start time (estimated)
	batch.StartTime = v.estimateSlotTime(batch.StartSlot, currentSlot)

	return batch
}

// calculateTVCForLatency calculates TVC credits based on vote latency
// Formula from SIMD-0033: Latency 1-3 = 10 credits, then -1 credit per additional slot, minimum 1
func (v *VoteBatchAnalyzer) calculateTVCForLatency(latency int64) int64 {
	if latency <= TVCGracePeriod {
		// Grace period: latency 1-3 = max credits (10)
		return MaxTVCPerVote
	}

	// After grace period: 10 - (latency - 3)
	credits := MaxTVCPerVote - (latency - TVCGracePeriod)

	// Minimum is 1 credit (never 0)
	if credits < MinTVCPerVote {
		credits = MinTVCPerVote
	}

	return credits
}

// estimateAverageLatency estimates voting latency based on slot progression
func (v *VoteBatchAnalyzer) estimateAverageLatency(votes []Vote, currentSlot int64) float64 {
	if len(votes) == 0 {
		return 0.0
	}

	// Simplified latency calculation
	// In a real implementation, you'd compare actual timestamps with expected slot times

	var totalLatency float64
	slotTimeMs := 400.0 // ~400ms per slot average

	for _, vote := range votes {
		// Estimate how late this vote was submitted
		// This is a simplified calculation - real implementation would need more data
		expectedLatency := float64(vote.ConfirmationCount) * slotTimeMs / 1000.0
		if expectedLatency > MaxLatencySeconds {
			expectedLatency = MaxLatencySeconds
		}
		totalLatency += expectedLatency
	}

	return totalLatency / float64(len(votes))
}

// estimateSlotTime estimates when a slot occurred (simplified)
func (v *VoteBatchAnalyzer) estimateSlotTime(slot, currentSlot int64) time.Time {
	// Simplified: assume ~400ms per slot
	slotDiff := currentSlot - slot
	duration := time.Duration(slotDiff) * 400 * time.Millisecond
	return time.Now().Add(-duration)
}

// CollectVoteBatchMetrics collects and emits vote batch metrics
func (v *VoteBatchAnalyzer) CollectVoteBatchMetrics(
	ctx context.Context,
	ch chan<- prometheus.Metric,
	nodekey, votekey string,
) {
	// Get vote account data from cache
	voteAccountData, err := v.voteAccountCache.Get(ctx, votekey)
	if err != nil {
		v.logger.Errorf("Failed to get vote account info for %s: %v", votekey, err)
		return
	}

	// Analyze vote batches
	batches, err := v.AnalyzeVoteBatches(ctx, voteAccountData, nodekey, votekey)
	if err != nil {
		v.logger.Errorf("Failed to analyze vote batches for %s: %v", nodekey, err)
		return
	}

	// Emit metrics for each batch
	for _, batch := range batches {
		labels := []string{nodekey, votekey, fmt.Sprintf("%d", batch.ID), batch.SlotRange}

		v.VoteBatchMissedTVCs.WithLabelValues(labels...).Set(float64(batch.MissedTVCs))
		v.VoteBatchMissedSlots.WithLabelValues(labels...).Set(float64(batch.MissedSlots))
		v.VoteBatchAvgLatency.WithLabelValues(labels...).Set(batch.AvgLatency)
		v.VoteBatchPerformance.WithLabelValues(labels...).Set(batch.Performance)
		v.VoteBatchSlotRange.WithLabelValues(labels...).Set(float64(batch.TotalSlots))
	}

	// Emit total batch count
	v.VoteBatchCount.WithLabelValues(nodekey, votekey).Set(float64(len(batches)))

	v.logger.Infof("Collected vote batch metrics for %s: %d batches analyzed", nodekey, len(batches))
}

// GetVoteGaps identifies specific gaps in voting pattern
func (v *VoteBatchAnalyzer) GetVoteGaps(batches []VoteBatch) []VoteGap {
	var gaps []VoteGap

	for i := 0; i < len(batches)-1; i++ {
		currentBatch := batches[i]
		nextBatch := batches[i+1]

		gapStart := currentBatch.EndSlot + 1
		gapEnd := nextBatch.StartSlot - 1

		if gapEnd > gapStart {
			gap := VoteGap{
				StartSlot:  gapStart,
				EndSlot:    gapEnd,
				Duration:   gapEnd - gapStart + 1,
				Timestamp:  v.estimateSlotTime(gapStart, batches[len(batches)-1].EndSlot),
				MissedTVCs: gapEnd - gapStart + 1, // Simplified calculation
			}
			gaps = append(gaps, gap)
		}
	}

	return gaps
}

// Describe implements prometheus.Collector
func (v *VoteBatchAnalyzer) Describe(ch chan<- *prometheus.Desc) {
	v.VoteBatchMissedTVCs.Describe(ch)
	v.VoteBatchMissedSlots.Describe(ch)
	v.VoteBatchAvgLatency.Describe(ch)
	v.VoteBatchPerformance.Describe(ch)
	v.VoteBatchSlotRange.Describe(ch)
	v.VoteBatchCount.Describe(ch)
}

// Collect implements prometheus.Collector
func (v *VoteBatchAnalyzer) Collect(ch chan<- prometheus.Metric) {
	v.VoteBatchMissedTVCs.Collect(ch)
	v.VoteBatchMissedSlots.Collect(ch)
	v.VoteBatchAvgLatency.Collect(ch)
	v.VoteBatchPerformance.Collect(ch)
	v.VoteBatchSlotRange.Collect(ch)
	v.VoteBatchCount.Collect(ch)
}
