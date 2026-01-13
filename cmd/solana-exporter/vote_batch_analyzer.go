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
	BatchLabel         = "batch"
	SlotRangeLabel     = "slot_range"
	LatencyLabel       = "avg_latency"
	MissedTVCsLabel    = "missed_tvcs"
	MissedSlotsLabel   = "missed_slots"
	StartTimeLabel     = "start_time"

	// Vote batch analysis parameters
	MaxVoteBatchGap    = 50  // Maximum slots between votes in same batch
	MinBatchSize       = 100 // Minimum slots to consider as a batch
	MaxLatencySeconds  = 30  // Maximum expected voting latency
)

type (
	// VoteBatch represents a group of consecutive votes
	VoteBatch struct {
		ID             int       `json:"batch_id"`
		StartTime      time.Time `json:"start_time"`
		StartSlot      int64     `json:"start_slot"`
		EndSlot        int64     `json:"end_slot"`
		SlotRange      string    `json:"slot_range"`
		AvgLatency     float64   `json:"avg_latency"`
		MissedTVCs     int64     `json:"missed_tvcs"`
		MissedSlots    int64     `json:"missed_slots"`
		TotalSlots     int64     `json:"total_slots"`
		VotedSlots     int64     `json:"voted_slots"`
		Votes          []Vote `json:"votes"`
		Performance    float64   `json:"performance_pct"`
	}

	// VoteBatchAnalyzer analyzes vote patterns and detects gaps
	VoteBatchAnalyzer struct {
		rpcClient        *rpc.Client
		logger           *zap.SugaredLogger
		config           *ExporterConfig
		voteAccountCache *VoteAccountCache

		// Metrics for vote batch analysis
		VoteBatchMissedTVCs     *prometheus.GaugeVec
		VoteBatchMissedSlots    *prometheus.GaugeVec
		VoteBatchAvgLatency     *prometheus.GaugeVec
		VoteBatchPerformance    *prometheus.GaugeVec
		VoteBatchSlotRange      *prometheus.GaugeVec
		VoteBatchCount          *prometheus.GaugeVec
	}

	// VoteGap represents a gap in voting pattern
	VoteGap struct {
		StartSlot   int64     `json:"start_slot"`
		EndSlot     int64     `json:"end_slot"`
		Duration    int64     `json:"duration_slots"`
		Timestamp   time.Time `json:"timestamp"`
		MissedTVCs  int64     `json:"missed_tvcs"`
	}

	// Vote represents a validator vote (local copy from rpc package)
	Vote struct {
		ConfirmationCount int64 `json:"confirmationCount"`
		Slot              int64 `json:"slot"`
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

	batches := v.groupVotesIntoBatches(votes, currentSlot)
	v.logger.Debugf("Analyzed %d vote batches for validator %s", len(batches), nodekey)

	return batches, nil
}

// groupVotesIntoBatches groups consecutive votes into logical batches
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
