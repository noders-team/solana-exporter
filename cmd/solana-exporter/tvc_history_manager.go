package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	// Storage configuration
	TVCHistoryInterval    = 20000 // Store data every 20,000 slots
	MaxEpochsToKeep      = 5      // Keep last 5 epochs
	StorageDir           = "tvc_history"
	HistoryFileName      = "tvc_history.json"

	// Performance settings
	MaxHistoryPointsInMemory = 1000 // Max points to keep in memory
	SaveInterval             = 5 * time.Minute // Save to disk every 5 minutes
)

type (
	// TVCHistoryPoint represents a single TVC measurement point
	TVCHistoryPoint struct {
		Timestamp        time.Time `json:"timestamp"`
		Slot             int64     `json:"slot"`
		Epoch            int64     `json:"epoch"`
		SlotInEpoch      int64     `json:"slot_in_epoch"`
		EpochProgress    float64   `json:"epoch_progress"`
		VoteCredits      int64     `json:"vote_credits"`
		ExpectedCredits  int64     `json:"expected_credits"`
		MissedCredits    int64     `json:"missed_credits"`
		MissedPercent    float64   `json:"missed_percent"`
		Performance      float64   `json:"performance"`
		VoteLag          int64     `json:"vote_lag"`
		IsEpochEnd       bool      `json:"is_epoch_end"`
		BatchCount       int       `json:"batch_count"`
		AvgBatchLatency  float64   `json:"avg_batch_latency"`
		ValidatorActive  bool      `json:"validator_active"`
		NodeHealth       bool      `json:"node_health"`
	}

	// EpochSummary contains complete epoch performance data
	EpochSummary struct {
		Epoch              int64                `json:"epoch"`
		StartTime          time.Time            `json:"start_time"`
		EndTime            time.Time            `json:"end_time"`
		Duration           time.Duration        `json:"duration"`
		StartSlot          int64                `json:"start_slot"`
		EndSlot            int64                `json:"end_slot"`
		TotalSlots         int64                `json:"total_slots"`
		TotalVoteCredits   int64                `json:"total_vote_credits"`
		ExpectedCredits    int64                `json:"expected_credits"`
		MissedCredits      int64                `json:"missed_credits"`
		PerformancePercent float64              `json:"performance_percent"`
		HistoryPoints      []TVCHistoryPoint    `json:"history_points"`
		BatchesAnalyzed    int                  `json:"batches_analyzed"`
		AvgVoteLag         float64              `json:"avg_vote_lag"`
		MaxVoteLag         int64                `json:"max_vote_lag"`
		UptimePercent      float64              `json:"uptime_percent"`
		Grade              string               `json:"grade"` // A+, A, B, C, D, F
		// NEW: Vote batch historical data
		VoteBatches        []VoteBatchHistory   `json:"vote_batches"`
		BatchTrends        BatchTrendAnalysis   `json:"batch_trends"`
	}

	// VoteBatchHistory represents a saved vote batch with timestamp
	VoteBatchHistory struct {
		Timestamp       time.Time `json:"timestamp"`
		Slot            int64     `json:"slot"`
		Epoch           int64     `json:"epoch"`
		BatchID         int       `json:"batch_id"`
		StartSlot       int64     `json:"start_slot"`
		EndSlot         int64     `json:"end_slot"`
		SlotRange       string    `json:"slot_range"`
		TotalSlots      int64     `json:"total_slots"`
		VotedSlots      int64     `json:"voted_slots"`
		MissedSlots     int64     `json:"missed_slots"`
		MissedTVCs      int64     `json:"missed_tvcs"`
		Performance     float64   `json:"performance"`
		AvgLatency      float64   `json:"avg_latency"`
		VoteCount       int       `json:"vote_count"`
		IsComplete      bool      `json:"is_complete"`
		GapFromPrevious int64     `json:"gap_from_previous"`
	}

	// BatchTrendAnalysis contains trend analysis for vote batches
	BatchTrendAnalysis struct {
		TotalBatches        int     `json:"total_batches"`
		AvgBatchPerformance float64 `json:"avg_batch_performance"`
		AvgBatchLatency     float64 `json:"avg_batch_latency"`
		AvgBatchSize        float64 `json:"avg_batch_size"`
		AvgGapBetweenBatch  float64 `json:"avg_gap_between_batches"`
		PerfectBatches      int     `json:"perfect_batches"`      // 100% performance
		PoorBatches         int     `json:"poor_batches"`         // <90% performance
		HighLatencyBatches  int     `json:"high_latency_batches"` // >10s latency
		TrendDirection      string  `json:"trend_direction"`      // improving, degrading, stable
		LastAnalyzedSlot    int64   `json:"last_analyzed_slot"`
	}

	// TVCHistoryManager manages TVC data collection and storage
	TVCHistoryManager struct {
		rpcClient         *rpc.Client
		voteBatchAnalyzer *VoteBatchAnalyzer
		voteAccountCache  *VoteAccountCache
		config            *ExporterConfig
		logger            *zap.SugaredLogger

		// Data storage
		currentEpochData  *EpochSummary
		historicalEpochs  map[int64]*EpochSummary
		realtimePoints    []TVCHistoryPoint
		lastStoredSlot    int64
		lastSaveTime      time.Time

		// NEW: Batch caching and persistence
		cachedBatches     []VoteBatchHistory
		lastBatchSlot     int64
		batchSaveInterval time.Duration

		// Synchronization
		mu                sync.RWMutex

		// Metrics
		TVCHistoryPoints      *prometheus.GaugeVec
		EpochPerformanceGrade *prometheus.GaugeVec
		HistoryStorageSize    prometheus.Gauge
		// NEW: Batch metrics
		BatchHistoryCount     *prometheus.GaugeVec
		BatchTrendMetrics     *prometheus.GaugeVec
	}

	// HistoryQueryRequest for API requests
	HistoryQueryRequest struct {
		Epoch     *int64 `json:"epoch,omitempty"`
		StartSlot *int64 `json:"start_slot,omitempty"`
		EndSlot   *int64 `json:"end_slot,omitempty"`
		Limit     *int   `json:"limit,omitempty"`
	}

	// HistoryResponse for API responses
	HistoryResponse struct {
		CurrentEpoch     *EpochSummary      `json:"current_epoch"`
		HistoricalEpochs []*EpochSummary    `json:"historical_epochs"`
		RealtimePoints   []TVCHistoryPoint  `json:"realtime_points"`
		TotalPoints      int                `json:"total_points"`
		LastUpdate       time.Time          `json:"last_update"`
		// NEW: Batch data in response
		RecentBatches    []VoteBatchHistory `json:"recent_batches"`
		BatchTrends      *BatchTrendAnalysis `json:"batch_trends"`
		TotalBatches     int                 `json:"total_batches"`
	}

	// BatchQueryRequest for batch-specific API requests
	BatchQueryRequest struct {
		Epoch      *int64 `json:"epoch,omitempty"`
		StartSlot  *int64 `json:"start_slot,omitempty"`
		EndSlot    *int64 `json:"end_slot,omitempty"`
		Limit      *int   `json:"limit,omitempty"`
		MinPerf    *float64 `json:"min_performance,omitempty"`
		MaxLatency *float64 `json:"max_latency,omitempty"`
	}
)

func NewTVCHistoryManager(rpcClient *rpc.Client, voteBatchAnalyzer *VoteBatchAnalyzer, config *ExporterConfig, voteAccountCache *VoteAccountCache) *TVCHistoryManager {
	logger := slog.Get()

	manager := &TVCHistoryManager{
		rpcClient:         rpcClient,
		voteBatchAnalyzer: voteBatchAnalyzer,
		voteAccountCache:  voteAccountCache,
		config:            config,
		logger:            logger,
		historicalEpochs:  make(map[int64]*EpochSummary),
		realtimePoints:    make([]TVCHistoryPoint, 0, MaxHistoryPointsInMemory),
		lastSaveTime:      time.Now(),
		// NEW: Initialize batch storage
		cachedBatches:     make([]VoteBatchHistory, 0, 500), // Cache last 500 batches
		batchSaveInterval: 10 * time.Minute,                 // Save batches every 10 minutes

		// Prometheus metrics
		TVCHistoryPoints: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_tvc_history_points",
				Help: "TVC performance history points with detailed tracking",
			},
			[]string{"nodekey", "votekey", "epoch", "metric_type"},
		),

		EpochPerformanceGrade: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_epoch_performance_grade",
				Help: "Epoch performance grade (A+=4.0, A=3.7, B=3.3, C=2.0, D=1.0, F=0.0)",
			},
			[]string{"nodekey", "votekey", "epoch", "grade"},
		),

		HistoryStorageSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "solana_tvc_history_storage_size_bytes",
				Help: "Size of TVC history storage on disk in bytes",
			},
		),

		// NEW: Batch metrics
		BatchHistoryCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_history_count",
				Help: "Number of vote batches stored in history",
			},
			[]string{"nodekey", "votekey", "epoch"},
		),

		BatchTrendMetrics: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "solana_vote_batch_trends",
				Help: "Vote batch trend analysis metrics",
			},
			[]string{"nodekey", "votekey", "metric_type"},
		),
	}

	// Create storage directory
	if err := os.MkdirAll(StorageDir, 0755); err != nil {
		logger.Errorf("Failed to create storage directory: %v", err)
	}

	// Load existing data
	manager.loadFromDisk()

	return manager
}

// StartHistoryCollection begins the TVC history collection process
func (t *TVCHistoryManager) StartHistoryCollection(ctx context.Context) {
	t.logger.Info("Starting TVC history collection...")

	// Collection ticker - every 30 seconds
	collectionTicker := time.NewTicker(30 * time.Second)
	defer collectionTicker.Stop()

	// Save ticker - every 5 minutes
	saveTicker := time.NewTicker(SaveInterval)
	defer saveTicker.Stop()

	// NEW: Batch collection ticker - every 2 minutes
	batchTicker := time.NewTicker(2 * time.Minute)
	defer batchTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.logger.Info("Stopping TVC history collection...")
			t.saveToDisk() // Final save
			return

		case <-collectionTicker.C:
			t.collectCurrentData(ctx)

		case <-saveTicker.C:
			t.saveToDisk()

		case <-batchTicker.C:
			t.collectAndStoreBatches(ctx)
		}
	}
}

// collectAndStoreBatches collects current vote batches and stores them
func (t *TVCHistoryManager) collectAndStoreBatches(ctx context.Context) {
	if len(t.config.Nodekeys) == 0 || len(t.config.Votekeys) == 0 {
		return
	}

	nodekey := t.config.Nodekeys[0]
	votekey := t.config.Votekeys[0]

	// Get current slot and epoch info
	currentSlot, err := t.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		t.logger.Errorf("Failed to get current slot for batch collection: %v", err)
		return
	}

	epochInfo, err := t.rpcClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		t.logger.Errorf("Failed to get epoch info for batch collection: %v", err)
		return
	}

	// Get vote account data from cache
	voteAccountData, err := t.voteAccountCache.Get(ctx, votekey)
	if err != nil {
		t.logger.Errorf("Failed to get vote account data for batch collection: %v", err)
		return
	}

	// Analyze vote batches
	batches, err := t.voteBatchAnalyzer.AnalyzeVoteBatches(ctx, voteAccountData, nodekey, votekey)
	if err != nil {
		t.logger.Errorf("Failed to analyze vote batches: %v", err)
		return
	}

	// Convert and store batches
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	var newBatches []VoteBatchHistory

	for _, batch := range batches {
		// Only store COMPLETED batches (20000 slots) that we haven't seen before
		// Skip live/incomplete batches (they will be shown from current analysis)
		const fixedBatchSize = 20000
		if batch.TotalSlots == fixedBatchSize && batch.EndSlot < currentSlot-10 {
			// Check if we already have this batch (by BatchID and StartSlot)
			alreadyExists := false
			for _, existing := range t.cachedBatches {
				if existing.BatchID == batch.ID && existing.StartSlot == batch.StartSlot {
					alreadyExists = true
					break
				}
			}

			if !alreadyExists {
				gapFromPrevious := int64(0)
				if len(t.cachedBatches) > 0 {
					lastBatch := t.cachedBatches[len(t.cachedBatches)-1]
					gapFromPrevious = batch.StartSlot - lastBatch.EndSlot
				}

				batchHistory := VoteBatchHistory{
					Timestamp:       now,
					Slot:            currentSlot,
					Epoch:           epochInfo.Epoch,
					BatchID:         batch.ID,
					StartSlot:       batch.StartSlot,
					EndSlot:         batch.EndSlot,
					SlotRange:       batch.SlotRange,
					TotalSlots:      batch.TotalSlots,
					VotedSlots:      batch.VotedSlots,
					MissedSlots:     batch.MissedSlots,
					MissedTVCs:      batch.MissedTVCs,
					Performance:     batch.Performance,
					AvgLatency:      batch.AvgLatency,
					VoteCount:       len(batch.Votes),
					IsComplete:      true, // Only completed batches are stored
					GapFromPrevious: gapFromPrevious,
				}

				newBatches = append(newBatches, batchHistory)
			}
		}
	}

	// Add new batches to cache
	t.cachedBatches = append(t.cachedBatches, newBatches...)

	// Maintain cache size limit
	if len(t.cachedBatches) > 500 {
		// Keep only the latest 500 batches
		copy(t.cachedBatches, t.cachedBatches[len(t.cachedBatches)-500:])
		t.cachedBatches = t.cachedBatches[:500]
	}

	// Update last batch slot (only for completed batches)
	if len(newBatches) > 0 {
		t.lastBatchSlot = newBatches[len(newBatches)-1].EndSlot
	}

	// Add batches to current epoch
	if t.currentEpochData != nil && epochInfo.Epoch == t.currentEpochData.Epoch {
		t.currentEpochData.VoteBatches = append(t.currentEpochData.VoteBatches, newBatches...)
	}

	// Emit batch metrics
	t.emitBatchMetrics(newBatches, nodekey, votekey)

	if len(newBatches) > 0 {
		t.logger.Infof("Stored %d new vote batches (epoch %d, slots %d-%d)",
			len(newBatches), epochInfo.Epoch,
			newBatches[0].StartSlot, newBatches[len(newBatches)-1].EndSlot)
	}
}

// collectCurrentData collects current TVC data point
func (t *TVCHistoryManager) collectCurrentData(ctx context.Context) {
	if len(t.config.Nodekeys) == 0 || len(t.config.Votekeys) == 0 {
		return
	}

	nodekey := t.config.Nodekeys[0]
	votekey := t.config.Votekeys[0]

	// Get current epoch info
	epochInfo, err := t.rpcClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		t.logger.Errorf("Failed to get epoch info: %v", err)
		return
	}

	// Get current slot
	currentSlot, err := t.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		t.logger.Errorf("Failed to get current slot: %v", err)
		return
	}

	// Get vote accounts for TVC data
	voteAccounts, err := t.rpcClient.GetVoteAccounts(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		t.logger.Errorf("Failed to get vote accounts: %v", err)
		return
	}

	// Find our validator
	var validatorAccount *rpc.VoteAccount
	for _, account := range append(voteAccounts.Current, voteAccounts.Delinquent...) {
		if account.NodePubkey == nodekey || account.VotePubkey == votekey {
			validatorAccount = &account
			break
		}
	}

	if validatorAccount == nil {
		t.logger.Warn("Validator not found in vote accounts")
		return
	}

	// Get detailed vote account data from cache
	voteAccountData, err := t.voteAccountCache.Get(ctx, votekey)
	if err != nil {
		t.logger.Errorf("Failed to get vote account info: %v", err)
		return
	}

	// Calculate TVC data
	point := t.calculateTVCPoint(epochInfo, currentSlot, validatorAccount, voteAccountData)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if we need to start a new epoch
	if t.currentEpochData == nil || t.currentEpochData.Epoch != epochInfo.Epoch {
		t.finalizeCurrentEpoch()
		t.startNewEpoch(epochInfo, point)
	}

	// Add point to current epoch
	t.currentEpochData.HistoryPoints = append(t.currentEpochData.HistoryPoints, point)

	// Add to real-time points (with size limit)
	t.realtimePoints = append(t.realtimePoints, point)
	if len(t.realtimePoints) > MaxHistoryPointsInMemory {
		// Keep only the latest points
		copy(t.realtimePoints, t.realtimePoints[len(t.realtimePoints)-MaxHistoryPointsInMemory:])
		t.realtimePoints = t.realtimePoints[:MaxHistoryPointsInMemory]
	}

	// Check if this is an interval point (every 20,000 slots)
	if point.Slot-t.lastStoredSlot >= TVCHistoryInterval {
		t.lastStoredSlot = point.Slot
		t.logger.Infof("Stored TVC history point at slot %d (epoch %d, %.1f%% complete)",
			point.Slot, point.Epoch, point.EpochProgress)
	}

	// Update current epoch summary
	t.updateCurrentEpochSummary(point)

	// Emit Prometheus metrics
	t.emitHistoryMetrics(point, nodekey, votekey)
}

// calculateTVCPoint creates a TVC history point from current data
func (t *TVCHistoryManager) calculateTVCPoint(
	epochInfo *rpc.EpochInfo,
	currentSlot int64,
	validatorAccount *rpc.VoteAccount,
	voteAccountData *rpc.VoteAccountData,
) TVCHistoryPoint {

	// Calculate credits for current epoch
	var currentEpochCredits, previousEpochCredits int64
	for _, credit := range voteAccountData.EpochCredits {
		if credit.Epoch == epochInfo.Epoch {
			if credits, err := strconv.ParseInt(credit.Credits, 10, 64); err == nil {
				currentEpochCredits = credits
			}
			if prevCredits, err := strconv.ParseInt(credit.PreviousCredits, 10, 64); err == nil {
				previousEpochCredits = prevCredits
			}
			break
		}
	}

	voteCredits := currentEpochCredits - previousEpochCredits
	expectedCredits := epochInfo.SlotIndex + 1 // Simplified calculation
	missedCredits := expectedCredits - voteCredits
	if missedCredits < 0 {
		missedCredits = 0
	}

	var missedPercent, performance float64
	if expectedCredits > 0 {
		missedPercent = float64(missedCredits) / float64(expectedCredits) * 100.0
		performance = 100.0 - missedPercent
	}

	// Calculate vote lag
	voteLag := currentSlot - int64(validatorAccount.LastVote)

	// Get node health
	nodeHealth := true // Would need actual health check

	// Check if validator is active (not delinquent)
	validatorActive := true
	// This would be determined by checking delinquent list

	return TVCHistoryPoint{
		Timestamp:        time.Now(),
		Slot:             currentSlot,
		Epoch:            epochInfo.Epoch,
		SlotInEpoch:      epochInfo.SlotIndex,
		EpochProgress:    float64(epochInfo.SlotIndex) / float64(epochInfo.SlotsInEpoch) * 100.0,
		VoteCredits:      voteCredits,
		ExpectedCredits:  expectedCredits,
		MissedCredits:    missedCredits,
		MissedPercent:    missedPercent,
		Performance:      performance,
		VoteLag:          voteLag,
		IsEpochEnd:       epochInfo.SlotIndex+1 >= epochInfo.SlotsInEpoch,
		BatchCount:       len(voteAccountData.Votes), // Simplified
		AvgBatchLatency:  3.0, // Would calculate from actual batch data
		ValidatorActive:  validatorActive,
		NodeHealth:       nodeHealth,
	}
}

// startNewEpoch initializes a new epoch data structure
func (t *TVCHistoryManager) startNewEpoch(epochInfo *rpc.EpochInfo, firstPoint TVCHistoryPoint) {
	t.currentEpochData = &EpochSummary{
		Epoch:            epochInfo.Epoch,
		StartTime:        firstPoint.Timestamp,
		StartSlot:        epochInfo.AbsoluteSlot - epochInfo.SlotIndex,
		EndSlot:          epochInfo.AbsoluteSlot - epochInfo.SlotIndex + epochInfo.SlotsInEpoch - 1,
		TotalSlots:       epochInfo.SlotsInEpoch,
		HistoryPoints:    make([]TVCHistoryPoint, 0, int(epochInfo.SlotsInEpoch/TVCHistoryInterval)+10),
	}

	t.logger.Infof("Started tracking new epoch %d", epochInfo.Epoch)
}

	// finalizeCurrentEpoch completes the current epoch and archives it
	func (t *TVCHistoryManager) finalizeCurrentEpoch() {
		if t.currentEpochData == nil {
			return
		}

		// Calculate final epoch statistics
		t.calculateEpochStatistics(t.currentEpochData)

		// NEW: Calculate batch trend analysis for the epoch
		t.currentEpochData.BatchTrends = t.calculateBatchTrends(t.currentEpochData.VoteBatches)

		// Archive the epoch
		t.historicalEpochs[t.currentEpochData.Epoch] = t.currentEpochData

		// Clean up old epochs (keep only last 5)
		t.cleanupOldEpochs()

		t.logger.Infof("Finalized epoch %d: %.2f%% performance (%s grade), %d batches analyzed",
			t.currentEpochData.Epoch,
			t.currentEpochData.PerformancePercent,
			t.currentEpochData.Grade,
			len(t.currentEpochData.VoteBatches))
	}

	// calculateBatchTrends analyzes batch trends for an epoch
	func (t *TVCHistoryManager) calculateBatchTrends(batches []VoteBatchHistory) BatchTrendAnalysis {
		if len(batches) == 0 {
			return BatchTrendAnalysis{TrendDirection: "no_data"}
		}

		trends := BatchTrendAnalysis{
			TotalBatches: len(batches),
		}

		// Calculate averages
		var totalPerformance, totalLatency, totalSize, totalGap float64
		var perfectCount, poorCount, highLatencyCount int

		for i, batch := range batches {
			totalPerformance += batch.Performance
			totalLatency += batch.AvgLatency
			totalSize += float64(batch.TotalSlots)

			if i > 0 {
				totalGap += float64(batch.GapFromPrevious)
			}

			if batch.Performance >= 100.0 {
				perfectCount++
			} else if batch.Performance < 90.0 {
				poorCount++
			}

			if batch.AvgLatency > 10.0 {
				highLatencyCount++
			}

			trends.LastAnalyzedSlot = batch.EndSlot
		}

		trends.AvgBatchPerformance = totalPerformance / float64(len(batches))
		trends.AvgBatchLatency = totalLatency / float64(len(batches))
		trends.AvgBatchSize = totalSize / float64(len(batches))
		trends.PerfectBatches = perfectCount
		trends.PoorBatches = poorCount
		trends.HighLatencyBatches = highLatencyCount

		if len(batches) > 1 {
			trends.AvgGapBetweenBatch = totalGap / float64(len(batches)-1)
		}

		// Determine trend direction (simple analysis)
		if len(batches) >= 3 {
			recentPerf := (batches[len(batches)-1].Performance + batches[len(batches)-2].Performance) / 2
			olderPerf := (batches[0].Performance + batches[1].Performance) / 2

			if recentPerf > olderPerf+2 {
				trends.TrendDirection = "improving"
			} else if recentPerf < olderPerf-2 {
				trends.TrendDirection = "degrading"
			} else {
				trends.TrendDirection = "stable"
			}
		} else {
			trends.TrendDirection = "insufficient_data"
		}

		return trends
	}

// calculateEpochStatistics computes final statistics for an epoch
func (t *TVCHistoryManager) calculateEpochStatistics(epoch *EpochSummary) {
	if len(epoch.HistoryPoints) == 0 {
		return
	}

	lastPoint := epoch.HistoryPoints[len(epoch.HistoryPoints)-1]
	epoch.EndTime = lastPoint.Timestamp
	epoch.Duration = epoch.EndTime.Sub(epoch.StartTime)
	epoch.TotalVoteCredits = lastPoint.VoteCredits
	epoch.ExpectedCredits = lastPoint.ExpectedCredits
	epoch.MissedCredits = lastPoint.MissedCredits
	epoch.PerformancePercent = lastPoint.Performance
	epoch.BatchesAnalyzed = len(epoch.HistoryPoints)

	// Calculate average and max vote lag
	var totalVoteLag, maxVoteLag, uptimeCount int64
	for _, point := range epoch.HistoryPoints {
		totalVoteLag += point.VoteLag
		if point.VoteLag > maxVoteLag {
			maxVoteLag = point.VoteLag
		}
		if point.ValidatorActive && point.NodeHealth {
			uptimeCount++
		}
	}

	if len(epoch.HistoryPoints) > 0 {
		epoch.AvgVoteLag = float64(totalVoteLag) / float64(len(epoch.HistoryPoints))
		epoch.UptimePercent = float64(uptimeCount) / float64(len(epoch.HistoryPoints)) * 100.0
	}
	epoch.MaxVoteLag = maxVoteLag

	// Assign performance grade
	epoch.Grade = t.calculatePerformanceGrade(epoch.PerformancePercent, epoch.UptimePercent)
}

// calculatePerformanceGrade assigns a letter grade based on performance
func (t *TVCHistoryManager) calculatePerformanceGrade(performance, uptime float64) string {
	// Weighted score: 80% performance + 20% uptime
	score := (performance * 0.8) + (uptime * 0.2)

	switch {
	case score >= 99.0:
		return "A+"
	case score >= 97.0:
		return "A"
	case score >= 94.0:
		return "A-"
	case score >= 90.0:
		return "B+"
	case score >= 87.0:
		return "B"
	case score >= 84.0:
		return "B-"
	case score >= 80.0:
		return "C+"
	case score >= 77.0:
		return "C"
	case score >= 70.0:
		return "C-"
	case score >= 65.0:
		return "D+"
	case score >= 60.0:
		return "D"
	default:
		return "F"
	}
}

// updateCurrentEpochSummary updates the current epoch with new data
func (t *TVCHistoryManager) updateCurrentEpochSummary(point TVCHistoryPoint) {
	if t.currentEpochData == nil {
		return
	}

	// Update running totals
	t.currentEpochData.TotalVoteCredits = point.VoteCredits
	t.currentEpochData.ExpectedCredits = point.ExpectedCredits
	t.currentEpochData.MissedCredits = point.MissedCredits
	t.currentEpochData.PerformancePercent = point.Performance

	// Update max vote lag if necessary
	if point.VoteLag > t.currentEpochData.MaxVoteLag {
		t.currentEpochData.MaxVoteLag = point.VoteLag
	}
}

// cleanupOldEpochs removes epochs older than MaxEpochsToKeep
func (t *TVCHistoryManager) cleanupOldEpochs() {
	if len(t.historicalEpochs) <= MaxEpochsToKeep {
		return
	}

	// Get sorted epoch numbers
	epochs := make([]int64, 0, len(t.historicalEpochs))
	for epoch := range t.historicalEpochs {
		epochs = append(epochs, epoch)
	}
	sort.Slice(epochs, func(i, j int) bool { return epochs[i] < epochs[j] })

	// Remove oldest epochs
	toRemove := len(epochs) - MaxEpochsToKeep
	for i := 0; i < toRemove; i++ {
		epoch := epochs[i]
		delete(t.historicalEpochs, epoch)
		t.logger.Infof("Cleaned up old epoch data: %d", epoch)
	}
}

	// emitHistoryMetrics emits Prometheus metrics for TVC history
	func (t *TVCHistoryManager) emitHistoryMetrics(point TVCHistoryPoint, nodekey, votekey string) {
		epochStr := fmt.Sprintf("%d", point.Epoch)

		// Emit various metrics
		t.TVCHistoryPoints.WithLabelValues(nodekey, votekey, epochStr, "performance").Set(point.Performance)
		t.TVCHistoryPoints.WithLabelValues(nodekey, votekey, epochStr, "vote_lag").Set(float64(point.VoteLag))
		t.TVCHistoryPoints.WithLabelValues(nodekey, votekey, epochStr, "missed_percent").Set(point.MissedPercent)
		t.TVCHistoryPoints.WithLabelValues(nodekey, votekey, epochStr, "epoch_progress").Set(point.EpochProgress)

		// Emit epoch grade if epoch is complete
		if t.currentEpochData != nil && t.currentEpochData.Grade != "" {
			gradeValue := t.gradeToFloat(t.currentEpochData.Grade)
			t.EpochPerformanceGrade.WithLabelValues(nodekey, votekey, epochStr, t.currentEpochData.Grade).Set(gradeValue)
		}
	}

	// emitBatchMetrics emits Prometheus metrics for vote batches
	func (t *TVCHistoryManager) emitBatchMetrics(batches []VoteBatchHistory, nodekey, votekey string) {
		if len(batches) == 0 {
			return
		}

		// Count batches by epoch
		epochCounts := make(map[int64]int)
		for _, batch := range batches {
			epochCounts[batch.Epoch]++
		}

		// Emit batch counts per epoch
		for epoch, count := range epochCounts {
			t.BatchHistoryCount.WithLabelValues(nodekey, votekey, fmt.Sprintf("%d", epoch)).Set(float64(count))
		}

		// Calculate and emit trend metrics
		if t.currentEpochData != nil && len(t.currentEpochData.VoteBatches) > 0 {
			trends := t.calculateBatchTrends(t.currentEpochData.VoteBatches)

			t.BatchTrendMetrics.WithLabelValues(nodekey, votekey, "avg_performance").Set(trends.AvgBatchPerformance)
			t.BatchTrendMetrics.WithLabelValues(nodekey, votekey, "avg_latency").Set(trends.AvgBatchLatency)
			t.BatchTrendMetrics.WithLabelValues(nodekey, votekey, "avg_size").Set(trends.AvgBatchSize)
			t.BatchTrendMetrics.WithLabelValues(nodekey, votekey, "perfect_batches").Set(float64(trends.PerfectBatches))
			t.BatchTrendMetrics.WithLabelValues(nodekey, votekey, "poor_batches").Set(float64(trends.PoorBatches))
			t.BatchTrendMetrics.WithLabelValues(nodekey, votekey, "high_latency_batches").Set(float64(trends.HighLatencyBatches))
		}
	}

// gradeToFloat converts letter grade to numeric value for metrics
func (t *TVCHistoryManager) gradeToFloat(grade string) float64 {
	switch grade {
	case "A+":
		return 4.0
	case "A":
		return 3.7
	case "A-":
		return 3.3
	case "B+":
		return 3.0
	case "B":
		return 2.7
	case "B-":
		return 2.3
	case "C+":
		return 2.0
	case "C":
		return 1.7
	case "C-":
		return 1.3
	case "D+":
		return 1.0
	case "D":
		return 0.7
	default: // "F"
		return 0.0
	}
}

	// GetHistoryData returns TVC history data for API requests
	func (t *TVCHistoryManager) GetHistoryData(req HistoryQueryRequest) (*HistoryResponse, error) {
		t.mu.RLock()
		defer t.mu.RUnlock()

		response := &HistoryResponse{
			CurrentEpoch:     t.currentEpochData,
			HistoricalEpochs: make([]*EpochSummary, 0, len(t.historicalEpochs)),
			RealtimePoints:   make([]TVCHistoryPoint, len(t.realtimePoints)),
			LastUpdate:       time.Now(),
		}

		// Copy realtime points
		copy(response.RealtimePoints, t.realtimePoints)

		// NEW: Add recent batches
		batchLimit := 50
		if req.Limit != nil && *req.Limit < batchLimit {
			batchLimit = *req.Limit
		}

		if len(t.cachedBatches) > 0 {
			startIdx := len(t.cachedBatches) - batchLimit
			if startIdx < 0 {
				startIdx = 0
			}
			response.RecentBatches = make([]VoteBatchHistory, len(t.cachedBatches)-startIdx)
			copy(response.RecentBatches, t.cachedBatches[startIdx:])
		}

		response.TotalBatches = len(t.cachedBatches)

		// Add batch trends from current epoch
		if t.currentEpochData != nil && len(t.currentEpochData.VoteBatches) > 0 {
			trends := t.calculateBatchTrends(t.currentEpochData.VoteBatches)
			response.BatchTrends = &trends
		}

		// Add historical epochs (sorted by epoch number)
		epochs := make([]int64, 0, len(t.historicalEpochs))
		for epoch := range t.historicalEpochs {
			epochs = append(epochs, epoch)
		}
		sort.Slice(epochs, func(i, j int) bool { return epochs[i] > epochs[j] }) // Latest first

		for _, epoch := range epochs {
			if req.Epoch == nil || *req.Epoch == epoch {
				response.HistoricalEpochs = append(response.HistoricalEpochs, t.historicalEpochs[epoch])
			}
		}

		// Apply limit
		if req.Limit != nil && len(response.RealtimePoints) > *req.Limit {
			response.RealtimePoints = response.RealtimePoints[len(response.RealtimePoints)-*req.Limit:]
		}

		response.TotalPoints = len(response.RealtimePoints)
		if response.CurrentEpoch != nil {
			response.TotalPoints += len(response.CurrentEpoch.HistoryPoints)
		}
		for _, epoch := range response.HistoricalEpochs {
			response.TotalPoints += len(epoch.HistoryPoints)
		}

		return response, nil
	}

	// GetRecentBatches returns the most recent N batches from history
	func (t *TVCHistoryManager) GetRecentBatches(limit int) []VoteBatchHistory {
		t.mu.RLock()
		defer t.mu.RUnlock()

		if len(t.cachedBatches) == 0 {
			return []VoteBatchHistory{}
		}

		// Return the most recent batches (they are stored in chronological order)
		startIdx := len(t.cachedBatches) - limit
		if startIdx < 0 {
			startIdx = 0
		}

		result := make([]VoteBatchHistory, len(t.cachedBatches)-startIdx)
		copy(result, t.cachedBatches[startIdx:])
		
		// Reverse to show newest first
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}

		return result
	}

	// GetBatchHistory returns vote batch history for API requests
	func (t *TVCHistoryManager) GetBatchHistory(req BatchQueryRequest) ([]VoteBatchHistory, *BatchTrendAnalysis, error) {
		t.mu.RLock()
		defer t.mu.RUnlock()

		var filteredBatches []VoteBatchHistory

		// Apply filters
		for _, batch := range t.cachedBatches {
			// Filter by epoch
			if req.Epoch != nil && batch.Epoch != *req.Epoch {
				continue
			}

			// Filter by slot range
			if req.StartSlot != nil && batch.EndSlot < *req.StartSlot {
				continue
			}
			if req.EndSlot != nil && batch.StartSlot > *req.EndSlot {
				continue
			}

			// Filter by performance
			if req.MinPerf != nil && batch.Performance < *req.MinPerf {
				continue
			}

			// Filter by latency
			if req.MaxLatency != nil && batch.AvgLatency > *req.MaxLatency {
				continue
			}

			filteredBatches = append(filteredBatches, batch)
		}

		// Apply limit
		if req.Limit != nil && len(filteredBatches) > *req.Limit {
			// Keep the most recent batches
			startIdx := len(filteredBatches) - *req.Limit
			filteredBatches = filteredBatches[startIdx:]
		}

		// Calculate trends for filtered batches
		var trends *BatchTrendAnalysis
		if len(filteredBatches) > 0 {
			trendAnalysis := t.calculateBatchTrends(filteredBatches)
			trends = &trendAnalysis
		}

		return filteredBatches, trends, nil
	}

// saveToDisk saves TVC history data to disk
func (t *TVCHistoryManager) saveToDisk() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Prepare data for saving
	saveData := struct {
		CurrentEpoch     *EpochSummary              `json:"current_epoch"`
		HistoricalEpochs map[int64]*EpochSummary    `json:"historical_epochs"`
		RealtimePoints   []TVCHistoryPoint          `json:"realtime_points"`
		CachedBatches    []VoteBatchHistory         `json:"cached_batches"`
		LastStoredSlot   int64                      `json:"last_stored_slot"`
		LastBatchSlot    int64                      `json:"last_batch_slot"`
		SavedAt          time.Time                  `json:"saved_at"`
	}{
		CurrentEpoch:     t.currentEpochData,
		HistoricalEpochs: t.historicalEpochs,
		RealtimePoints:   t.realtimePoints,
		CachedBatches:    t.cachedBatches,
		LastStoredSlot:   t.lastStoredSlot,
		LastBatchSlot:    t.lastBatchSlot,
		SavedAt:          time.Now(),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(saveData, "", "  ")
	if err != nil {
		t.logger.Errorf("Failed to marshal TVC history data: %v", err)
		return
	}

	// Write to file
	filePath := filepath.Join(StorageDir, HistoryFileName)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.logger.Errorf("Failed to save TVC history to disk: %v", err)
		return
	}

	// Update metrics
	t.HistoryStorageSize.Set(float64(len(data)))

	t.logger.Debugf("Saved TVC history to disk (%d bytes)", len(data))
}

// loadFromDisk loads TVC history data from disk
func (t *TVCHistoryManager) loadFromDisk() {
	filePath := filepath.Join(StorageDir, HistoryFileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			t.logger.Errorf("Failed to read TVC history from disk: %v", err)
		}
		return
	}

	// Unmarshal data
	var saveData struct {
		CurrentEpoch     *EpochSummary              `json:"current_epoch"`
		HistoricalEpochs map[int64]*EpochSummary    `json:"historical_epochs"`
		RealtimePoints   []TVCHistoryPoint          `json:"realtime_points"`
		CachedBatches    []VoteBatchHistory         `json:"cached_batches"`
		LastStoredSlot   int64                      `json:"last_stored_slot"`
		LastBatchSlot    int64                      `json:"last_batch_slot"`
		SavedAt          time.Time                  `json:"saved_at"`
	}

	if err := json.Unmarshal(data, &saveData); err != nil {
		t.logger.Errorf("Failed to unmarshal TVC history data: %v", err)
		return
	}

	// Load data
	t.mu.Lock()
	defer t.mu.Unlock()

	t.currentEpochData = saveData.CurrentEpoch
	if saveData.HistoricalEpochs != nil {
		t.historicalEpochs = saveData.HistoricalEpochs
	}
	if saveData.RealtimePoints != nil {
		t.realtimePoints = saveData.RealtimePoints
	}
	if saveData.CachedBatches != nil {
		t.cachedBatches = saveData.CachedBatches
	}
	t.lastStoredSlot = saveData.LastStoredSlot
	t.lastBatchSlot = saveData.LastBatchSlot

	t.logger.Infof("Loaded TVC history from disk: %d historical epochs, %d realtime points, %d cached batches",
		len(t.historicalEpochs), len(t.realtimePoints), len(t.cachedBatches))
}

// Describe implements prometheus.Collector interface
func (t *TVCHistoryManager) Describe(ch chan<- *prometheus.Desc) {
	t.TVCHistoryPoints.Describe(ch)
	t.EpochPerformanceGrade.Describe(ch)
	t.HistoryStorageSize.Describe(ch)
	t.BatchHistoryCount.Describe(ch)
	t.BatchTrendMetrics.Describe(ch)
}

// Collect implements prometheus.Collector interface
func (t *TVCHistoryManager) Collect(ch chan<- prometheus.Metric) {
	t.TVCHistoryPoints.Collect(ch)
	t.EpochPerformanceGrade.Collect(ch)
	t.HistoryStorageSize.Collect(ch)
	t.BatchHistoryCount.Collect(ch)
	t.BatchTrendMetrics.Collect(ch)
}
