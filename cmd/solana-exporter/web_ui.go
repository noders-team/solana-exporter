package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"go.uber.org/zap"
)

type WebUI struct {
	collector          *SolanaCollector
	voteBatchAnalyzer  *VoteBatchAnalyzer
	tvcHistoryManager  *TVCHistoryManager
	rpcClient          *rpc.Client
	config             *ExporterConfig
	logger             *zap.SugaredLogger
}

type DashboardData struct {
	NodeHealth        NodeHealthData        `json:"node_health"`
	ValidatorStatus   ValidatorStatusData   `json:"validator_status"`
	VoteCredits       VoteCreditsData       `json:"vote_credits"`
	VoteBatches       []VoteBatchData       `json:"vote_batches"`
	NetworkActivity   NetworkActivityData   `json:"network_activity"`
	Financial         FinancialData         `json:"financial"`
	LastUpdate        time.Time             `json:"last_update"`
}

type NodeHealthData struct {
	IsHealthy      bool    `json:"is_healthy"`
	SlotsBehind    int64   `json:"slots_behind"`
	IsActive       bool    `json:"is_active"`
	Version        string  `json:"version"`
	IsOutdated     bool    `json:"is_outdated"`
	LatestVersion  string  `json:"latest_version"`
}

type ValidatorStatusData struct {
	ActiveStake    float64 `json:"active_stake"`
	IsDelinquent   bool    `json:"is_delinquent"`
	Commission     float64 `json:"commission"`
	LastVote       int64   `json:"last_vote"`
	RootSlot       int64   `json:"root_slot"`
	Identity       string  `json:"identity"`
}

type VoteCreditsData struct {
	EpochCredits      int64   `json:"epoch_credits"`
	ExpectedCredits   int64   `json:"expected_credits"`
	MissedPercent     float64 `json:"missed_percent"`
	Performance       float64 `json:"performance"`
	VoteLag           int64   `json:"vote_lag"`
	CurrentEpoch      int64   `json:"current_epoch"`
}

type VoteBatchData struct {
	BatchID        int     `json:"batch_id"`
	SlotRange      string  `json:"slot_range"`
	StartTime      string  `json:"start_time"`
	MissedTVCs     int64   `json:"missed_tvcs"`
	MissedSlots    int64   `json:"missed_slots"`
	AvgLatency     float64 `json:"avg_latency"`
	Performance    float64 `json:"performance"`
	TotalSlots     int64   `json:"total_slots"`
	IsLive         bool    `json:"is_live"` // True for current batch being tracked in real-time
}

type NetworkActivityData struct {
	CurrentSlot      int64   `json:"current_slot"`
	CurrentEpoch     int64   `json:"current_epoch"`
	EpochFirstSlot   int64   `json:"epoch_first_slot"`
	EpochLastSlot    int64   `json:"epoch_last_slot"`
	EpochProgress    float64 `json:"epoch_progress"`
	TransactionsTotal int64  `json:"transactions_total"`
	BlockHeight      int64   `json:"block_height"`
}

type FinancialData struct {
	InflationRewards float64            `json:"inflation_rewards"`
	FeeRewards       float64            `json:"fee_rewards"`
	TotalRewards     float64            `json:"total_rewards"`
	AccountBalances  map[string]float64 `json:"account_balances"`
}

func NewWebUI(collector *SolanaCollector, voteBatchAnalyzer *VoteBatchAnalyzer, tvcHistoryManager *TVCHistoryManager, rpcClient *rpc.Client, config *ExporterConfig) *WebUI {
	return &WebUI{
		collector:         collector,
		voteBatchAnalyzer: voteBatchAnalyzer,
		tvcHistoryManager: tvcHistoryManager,
		rpcClient:         rpcClient,
		config:            config,
		logger:            slog.Get(),
	}
}

func (w *WebUI) RegisterHandlers(mux *http.ServeMux) {
	// Main dashboard
	mux.HandleFunc("/", w.handleDashboard)

	// API endpoints
	mux.HandleFunc("/api/dashboard", w.handleAPIDashboard)
	mux.HandleFunc("/api/health", w.handleAPIHealth)
	mux.HandleFunc("/api/vote-batches", w.handleAPIVoteBatches)
	mux.HandleFunc("/api/tvc-history", w.handleAPITVCHistory)
	mux.HandleFunc("/api/epoch-summary", w.handleAPIEpochSummary)
	mux.HandleFunc("/api/metrics/live", w.handleAPILiveMetrics)

	// Static assets (embedded)
	mux.HandleFunc("/static/", w.handleStatic)

	// Health check page
	mux.HandleFunc("/health", w.handleHealth)
}

func (w *WebUI) handleDashboard(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}

	dashboardHTML := w.getDashboardHTML()
	writer.Header().Set("Content-Type", "text/html")
	writer.Write([]byte(dashboardHTML))
}

func (w *WebUI) handleAPIDashboard(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := w.collectDashboardData(ctx)
	if err != nil {
		w.logger.Errorf("Failed to collect dashboard data: %v", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Access-Control-Allow-Origin", "*")

	if err := json.NewEncoder(writer).Encode(data); err != nil {
		w.logger.Errorf("Failed to encode dashboard data: %v", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (w *WebUI) handleAPIHealth(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := w.rpcClient.GetHealth(ctx)
	isHealthy := health == "ok" && err == nil

	response := map[string]interface{}{
		"healthy":    isHealthy,
		"timestamp":  time.Now().Unix(),
		"error":      nil,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(response)
}

func (w *WebUI) handleAPIVoteBatches(writer http.ResponseWriter, request *http.Request) {
	if len(w.config.Nodekeys) == 0 || len(w.config.Votekeys) == 0 {
		http.Error(writer, "No validators configured", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodekey := w.config.Nodekeys[0]
	votekey := w.config.Votekeys[0]

	// Get current live batch (real-time)
	var currentBatchData *VoteBatchData
	// Use vote account cache from collector to reduce RPC calls
	voteAccountData, err := w.collector.voteAccountCache.Get(ctx, votekey)
	if err != nil {
		w.logger.Errorf("Failed to get vote account info: %v", err)
		http.Error(writer, "Failed to get vote data", http.StatusInternalServerError)
		return
	}

	batches, err := w.voteBatchAnalyzer.AnalyzeVoteBatches(ctx, voteAccountData, nodekey, votekey)
	if err != nil {
		w.logger.Errorf("Failed to analyze vote batches: %v", err)
		http.Error(writer, "Failed to analyze vote batches", http.StatusInternalServerError)
		return
	}

	// Get the most recent batch as live batch
	if len(batches) > 0 {
		latestBatch := batches[len(batches)-1]
		currentBatchData = &VoteBatchData{
			BatchID:     latestBatch.ID,
			SlotRange:   latestBatch.SlotRange,
			StartTime:   latestBatch.StartTime.Format("02.01.2006 15:04:05"),
			MissedTVCs:  latestBatch.MissedTVCs,
			MissedSlots: latestBatch.MissedSlots,
			AvgLatency:  latestBatch.AvgLatency,
			Performance: latestBatch.Performance,
			TotalSlots:  latestBatch.TotalSlots,
			IsLive:      true, // Mark as live batch
		}
	}

	// Get historical batches from TVCHistoryManager
	historicalBatches := w.tvcHistoryManager.GetRecentBatches(50) // Get last 50 batches

	// Combine: live batch first, then historical batches
	var batchData []VoteBatchData
	
	// Add live batch as first row if available
	if currentBatchData != nil {
		batchData = append(batchData, *currentBatchData)
	}

	// Add historical batches (excluding the one that matches current live batch)
	for _, histBatch := range historicalBatches {
		// Skip if this historical batch matches the current live batch
		if currentBatchData != nil && histBatch.SlotRange == currentBatchData.SlotRange {
			continue
		}
		
		batchData = append(batchData, VoteBatchData{
			BatchID:     histBatch.BatchID,
			SlotRange:   histBatch.SlotRange,
			StartTime:   histBatch.Timestamp.Format("02.01.2006 15:04:05"),
			MissedTVCs:  histBatch.MissedTVCs,
			MissedSlots: histBatch.MissedSlots,
			AvgLatency:  histBatch.AvgLatency,
			Performance: histBatch.Performance,
			TotalSlots:  histBatch.TotalSlots,
			IsLive:      false, // Historical batch
		})
	}

	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(batchData)
}

func (w *WebUI) handleAPITVCHistory(writer http.ResponseWriter, request *http.Request) {
	// Parse query parameters
	query := request.URL.Query()

	req := HistoryQueryRequest{}

	if epochStr := query.Get("epoch"); epochStr != "" {
		if epoch, err := strconv.ParseInt(epochStr, 10, 64); err == nil {
			req.Epoch = &epoch
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = &limit
		}
	}

	// Get history data
	historyData, err := w.tvcHistoryManager.GetHistoryData(req)
	if err != nil {
		w.logger.Errorf("Failed to get TVC history: %v", err)
		http.Error(writer, "Failed to get TVC history", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(writer).Encode(historyData)
}

func (w *WebUI) handleAPIEpochSummary(writer http.ResponseWriter, request *http.Request) {
	// Get epoch summaries for the last 5 epochs
	req := HistoryQueryRequest{
		Limit: func() *int { l := 5; return &l }(),
	}

	historyData, err := w.tvcHistoryManager.GetHistoryData(req)
	if err != nil {
		w.logger.Errorf("Failed to get epoch summaries: %v", err)
		http.Error(writer, "Failed to get epoch summaries", http.StatusInternalServerError)
		return
	}

	// Return just the epoch summaries
	response := struct {
		CurrentEpoch     *EpochSummary   `json:"current_epoch"`
		HistoricalEpochs []*EpochSummary `json:"historical_epochs"`
		LastUpdate       time.Time       `json:"last_update"`
	}{
		CurrentEpoch:     historyData.CurrentEpoch,
		HistoricalEpochs: historyData.HistoricalEpochs,
		LastUpdate:       time.Now(),
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(writer).Encode(response)
}

func (w *WebUI) handleAPILiveMetrics(writer http.ResponseWriter, request *http.Request) {
	// Simple live metrics endpoint
	writer.Header().Set("Content-Type", "text/plain")
	writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Redirect to prometheus metrics for now
	http.Redirect(writer, request, "/metrics", http.StatusSeeOther)
}

func (w *WebUI) handleStatic(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/static/")

	switch {
	case strings.HasSuffix(path, ".css"):
		writer.Header().Set("Content-Type", "text/css")
		writer.Write([]byte(w.getMainCSS()))
	case strings.HasSuffix(path, ".js"):
		writer.Header().Set("Content-Type", "application/javascript")
		writer.Write([]byte(w.getMainJS()))
	default:
		http.NotFound(writer, request)
	}
}

func (w *WebUI) handleHealth(writer http.ResponseWriter, request *http.Request) {
	healthHTML := w.getHealthHTML()
	writer.Header().Set("Content-Type", "text/html")
	writer.Write([]byte(healthHTML))
}

func (w *WebUI) collectDashboardData(ctx context.Context) (*DashboardData, error) {
	data := &DashboardData{
		LastUpdate: time.Now(),
	}

	// Collect node health
	health, err := w.rpcClient.GetHealth(ctx)
	data.NodeHealth.IsHealthy = health == "ok" && err == nil

	// Get current slot for calculations
	currentSlot, err := w.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	if err == nil {
		data.NetworkActivity.CurrentSlot = currentSlot
	}

	// Get epoch info
	epochInfo, err := w.rpcClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err == nil {
		data.NetworkActivity.CurrentEpoch = epochInfo.Epoch
		data.NetworkActivity.EpochFirstSlot = epochInfo.AbsoluteSlot - epochInfo.SlotIndex
		data.NetworkActivity.EpochLastSlot = data.NetworkActivity.EpochFirstSlot + epochInfo.SlotsInEpoch - 1
		data.NetworkActivity.EpochProgress = float64(epochInfo.SlotIndex) / float64(epochInfo.SlotsInEpoch) * 100
		data.NetworkActivity.TransactionsTotal = epochInfo.TransactionCount
		data.NetworkActivity.BlockHeight = epochInfo.BlockHeight
	}

	// Collect validator data if configured
	if len(w.config.Nodekeys) > 0 && len(w.config.Votekeys) > 0 {
		err := w.collectValidatorData(ctx, data)
		if err != nil {
			w.logger.Warnf("Failed to collect validator data: %v", err)
		}
	}

	return data, nil
}

func (w *WebUI) collectValidatorData(ctx context.Context, data *DashboardData) error {
	nodekey := w.config.Nodekeys[0]
	votekey := w.config.Votekeys[0]

	// Get vote accounts
	voteAccounts, err := w.rpcClient.GetVoteAccounts(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return fmt.Errorf("failed to get vote accounts: %w", err)
	}

	// Find our validator
	var validatorAccount *rpc.VoteAccount
	for _, account := range append(voteAccounts.Current, voteAccounts.Delinquent...) {
		if account.NodePubkey == nodekey || account.VotePubkey == votekey {
			validatorAccount = &account
			break
		}
	}

	if validatorAccount != nil {
		data.ValidatorStatus.ActiveStake = float64(validatorAccount.ActivatedStake) / rpc.LamportsInSol
		data.ValidatorStatus.Commission = float64(validatorAccount.Commission)
		data.ValidatorStatus.LastVote = int64(validatorAccount.LastVote)
		data.ValidatorStatus.RootSlot = int64(validatorAccount.RootSlot)
		data.ValidatorStatus.Identity = nodekey

		// Check if delinquent
		for _, account := range voteAccounts.Delinquent {
			if account.NodePubkey == nodekey {
				data.ValidatorStatus.IsDelinquent = true
				break
			}
		}

		// Calculate vote lag
		if data.NetworkActivity.CurrentSlot > 0 {
			data.VoteCredits.VoteLag = data.NetworkActivity.CurrentSlot - data.ValidatorStatus.LastVote
		}
	}

	return nil
}

func (w *WebUI) getDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Solana Exporter Dashboard</title>
    <link rel="stylesheet" href="/static/main.css">
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🚀</text></svg>">
</head>
<body>
    <div id="app">
        <header class="header">
            <div class="container">
                <h1 class="logo">🚀 Solana Exporter</h1>
                <div class="header-stats">
                    <div class="stat" id="health-indicator">
                        <span class="stat-label">Status</span>
                        <span class="stat-value" id="health-status">Loading...</span>
                    </div>
                    <div class="stat">
                        <span class="stat-label">Last Update</span>
                        <span class="stat-value" id="last-update">--:--:--</span>
                    </div>
                </div>
            </div>
        </header>

        <main class="main">
            <div class="container">
                <!-- Critical Status Row -->
                <div class="dashboard-section">
                    <h2>🚨 Critical Status</h2>
                    <div class="cards-grid cards-grid-4">
                        <div class="card status-card" id="node-health-card">
                            <div class="card-header">
                                <h3>Node Health</h3>
                                <div class="status-indicator" id="health-indicator-dot"></div>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="node-health-value">--</div>
                                <div class="metric-subtitle" id="slots-behind">-- slots behind</div>
                            </div>
                        </div>

                        <div class="card status-card" id="validator-status-card">
                            <div class="card-header">
                                <h3>Validator Status</h3>
                                <div class="status-indicator" id="validator-indicator-dot"></div>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="validator-status-value">--</div>
                                <div class="metric-subtitle" id="validator-stake">-- SOL stake</div>
                            </div>
                        </div>

                        <div class="card status-card" id="vote-performance-card">
                            <div class="card-header">
                                <h3>Vote Performance</h3>
                                <div class="status-indicator" id="vote-indicator-dot"></div>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="vote-performance-value">--%</div>
                                <div class="metric-subtitle" id="vote-lag">-- slots lag</div>
                            </div>
                        </div>

                        <div class="card status-card" id="current-epoch-card">
                            <div class="card-header">
                                <h3>Current Epoch</h3>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="current-epoch-value">--</div>
                                <div class="metric-subtitle" id="epoch-progress">--% complete</div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Vote Batch Analysis -->
                <div class="dashboard-section">
                    <h2>🔍 Vote Batch Analysis</h2>
                    <div class="card">
                        <div class="card-header">
                            <h3>Recent Vote Batches</h3>
                            <button class="btn btn-small" id="refresh-batches">Refresh</button>
                        </div>
                        <div class="card-body">
                            <div class="table-container">
                                <table class="batch-table" id="batch-table">
                                    <thead>
                                        <tr>
                                            <th>Batch</th>
                                            <th>Start Time</th>
                                            <th>Slot Range</th>
                                            <th>Avg Latency</th>
                                            <th>Missed TVCs</th>
                                            <th>Missed Slots</th>
                                            <th>Performance</th>
                                        </tr>
                                    </thead>
                                    <tbody id="batch-table-body">
                                        <tr>
                                            <td colspan="7" class="loading-cell">Loading batch data...</td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- TVC History Section -->
                <div class="dashboard-section">
                    <h2>📈 TVC Performance History</h2>
                    <div class="cards-grid cards-grid-2">
                        <div class="card">
                            <div class="card-header">
                                <h3>Current Epoch Progress</h3>
                                <button class="btn btn-small" id="refresh-epoch">Refresh</button>
                            </div>
                            <div class="card-body">
                                <div class="epoch-progress-container">
                                    <div class="progress-bar">
                                        <div class="progress-fill" id="epoch-progress-bar"></div>
                                    </div>
                                    <div class="epoch-stats">
                                        <div class="stat-item">
                                            <span class="label">Performance:</span>
                                            <span class="value" id="current-performance">--%</span>
                                        </div>
                                        <div class="stat-item">
                                            <span class="label">Grade:</span>
                                            <span class="value grade" id="current-grade">--</span>
                                        </div>
                                        <div class="stat-item">
                                            <span class="label">Missed TVCs:</span>
                                            <span class="value" id="current-missed">--</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="card">
                            <div class="card-header">
                                <h3>Historical Performance (Last 5 Epochs)</h3>
                            </div>
                            <div class="card-body">
                                <div class="table-container">
                                    <table class="history-table" id="history-table">
                                        <thead>
                                            <tr>
                                                <th>Epoch</th>
                                                <th>Performance</th>
                                                <th>Grade</th>
                                                <th>Missed TVCs</th>
                                                <th>Duration</th>
                                            </tr>
                                        </thead>
                                        <tbody id="history-table-body">
                                            <tr>
                                                <td colspan="5" class="loading-cell">Loading history...</td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Network Activity -->
                <div class="dashboard-section">
                    <h2>📈 Network Activity</h2>
                    <div class="cards-grid cards-grid-3">
                        <div class="card">
                            <div class="card-header">
                                <h3>Current Slot</h3>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="current-slot">--</div>
                            </div>
                        </div>

                        <div class="card">
                            <div class="card-header">
                                <h3>Block Height</h3>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="block-height">--</div>
                            </div>
                        </div>

                        <div class="card">
                            <div class="card-header">
                                <h3>Transactions</h3>
                            </div>
                            <div class="card-body">
                                <div class="metric-large" id="transactions-total">--</div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Footer -->
                <footer class="footer">
                    <div class="footer-content">
                        <p>Solana Exporter Dashboard | <a href="/metrics" target="_blank">Prometheus Metrics</a> | <a href="/health">Health Check</a></p>
                        <p class="version">Enhanced with Vote Credits & Batch Analysis</p>
                    </div>
                </footer>
            </div>
        </main>
    </div>

    <script src="/static/main.js"></script>
</body>
</html>`
}

func (w *WebUI) getHealthHTML() string {
	return `<!DOCTYPE html>
<html>
<head>
    <title>Solana Exporter Health</title>
    <style>
        body { font-family: monospace; margin: 40px; background: #0d1117; color: #c9d1d9; }
        .health-ok { color: #238636; }
        .health-error { color: #da3633; }
        .metric { margin: 8px 0; }
    </style>
</head>
<body>
    <h1>🚀 Solana Exporter Health Check</h1>
    <div id="health-status">Loading...</div>

    <script>
        fetch('/api/health')
            .then(r => r.json())
            .then(data => {
                const status = document.getElementById('health-status');
                if (data.healthy) {
                    status.innerHTML = '<div class="health-ok">✅ Service is healthy</div>';
                } else {
                    status.innerHTML = '<div class="health-error">❌ Service is unhealthy</div>';
                    if (data.error) {
                        status.innerHTML += '<div>Error: ' + data.error + '</div>';
                    }
                }
                status.innerHTML += '<div class="metric">Timestamp: ' + new Date(data.timestamp * 1000) + '</div>';
                status.innerHTML += '<div class="metric"><a href="/">← Back to Dashboard</a></div>';
            })
            .catch(e => {
                document.getElementById('health-status').innerHTML = '<div class="health-error">❌ Failed to load: ' + e + '</div>';
            });
    </script>
</body>
</html>`
}

func (w *WebUI) getMainCSS() string {
	return `
/* Solana Exporter Dashboard CSS */
:root {
    --bg-primary: #0d1117;
    --bg-secondary: #161b22;
    --bg-tertiary: #21262d;
    --text-primary: #c9d1d9;
    --text-secondary: #8b949e;
    --text-accent: #58a6ff;
    --border-color: #30363d;
    --success-color: #238636;
    --warning-color: #d29922;
    --error-color: #da3633;
    --info-color: #58a6ff;
}

* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background-color: var(--bg-primary);
    color: var(--text-primary);
    line-height: 1.6;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 20px;
}

/* Header */
.header {
    background-color: var(--bg-secondary);
    border-bottom: 1px solid var(--border-color);
    padding: 1rem 0;
}

.header .container {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.logo {
    font-size: 1.5rem;
    font-weight: bold;
    color: var(--text-accent);
}

.header-stats {
    display: flex;
    gap: 2rem;
}

.stat {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
}

.stat-label {
    font-size: 0.875rem;
    color: var(--text-secondary);
}

.stat-value {
    font-weight: bold;
    color: var(--text-primary);
}

/* Main Content */
.main {
    padding: 2rem 0;
}

.dashboard-section {
    margin-bottom: 3rem;
}

.dashboard-section h2 {
    margin-bottom: 1rem;
    color: var(--text-primary);
    font-size: 1.25rem;
}

/* Cards */
.cards-grid {
    display: grid;
    gap: 1rem;
    margin-bottom: 2rem;
}

.cards-grid-4 {
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

.cards-grid-3 {
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
}

.card {
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    overflow: hidden;
}

.card-header {
    padding: 1rem;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
    background-color: var(--bg-tertiary);
}

.card-header h3 {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text-primary);
}

.card-body {
    padding: 1rem;
}

/* Status Cards */
.status-card {
    position: relative;
}

.status-indicator {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background-color: var(--text-secondary);
}

.status-indicator.healthy {
    background-color: var(--success-color);
    box-shadow: 0 0 8px var(--success-color);
}

.status-indicator.warning {
    background-color: var(--warning-color);
    box-shadow: 0 0 8px var(--warning-color);
}

.status-indicator.error {
    background-color: var(--error-color);
    box-shadow: 0 0 8px var(--error-color);
}

.metric-large {
    font-size: 2rem;
    font-weight: bold;
    color: var(--text-primary);
    margin-bottom: 0.5rem;
}

.metric-subtitle {
    font-size: 0.875rem;
    color: var(--text-secondary);
}

/* Buttons */
.btn {
    padding: 0.5rem 1rem;
    border: 1px solid var(--border-color);
    background-color: var(--bg-tertiary);
    color: var(--text-primary);
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.875rem;
    transition: all 0.2s;
}

.btn:hover {
    background-color: var(--bg-primary);
    border-color: var(--text-accent);
}

.btn-small {
    padding: 0.25rem 0.75rem;
    font-size: 0.75rem;
}

/* Table */
.table-container {
    overflow-x: auto;
}

.batch-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
}

.batch-table th,
.batch-table td {
    padding: 0.75rem;
    text-align: left;
    border-bottom: 1px solid var(--border-color);
}

.batch-table th {
    background-color: var(--bg-tertiary);
    font-weight: 600;
    color: var(--text-primary);
}

.batch-table td {
    color: var(--text-secondary);
}

.live-batch-row {
    background-color: rgba(88, 166, 255, 0.1);
    border-left: 3px solid var(--info-color);
}

.live-indicator {
    color: var(--info-color);
    font-size: 0.75rem;
    font-weight: bold;
    animation: pulse 2s infinite;
    margin-left: 0.5rem;
}

.loading-cell {
    text-align: center;
    padding: 2rem;
    color: var(--text-secondary);
}

.performance-good {
    color: var(--success-color);
}

.performance-warning {
    color: var(--warning-color);
}

.performance-error {
    color: var(--error-color);
}

/* Footer */
.footer {
    margin-top: 4rem;
    padding: 2rem 0;
    border-top: 1px solid var(--border-color);
}

.footer-content {
    text-align: center;
    color: var(--text-secondary);
    font-size: 0.875rem;
}

.footer-content a {
    color: var(--text-accent);
    text-decoration: none;
}

.footer-content a:hover {
    text-decoration: underline;
}

.version {
    margin-top: 0.5rem;
    font-size: 0.75rem;
}

/* Responsive */
@media (max-width: 768px) {
    .header .container {
        flex-direction: column;
        gap: 1rem;
    }

    .header-stats {
        gap: 1rem;
    }

    .cards-grid-4,
    .cards-grid-3 {
        grid-template-columns: 1fr;
    }

    .container {
        padding: 0 1rem;
    }
}

/* Animations */
@keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
}

.loading {
    animation: pulse 2s infinite;
}

/* TVC History Styles */
.cards-grid-2 {
    grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
}

.epoch-progress-container {
    padding: 1rem 0;
}

.progress-bar {
    width: 100%;
    height: 20px;
    background-color: var(--bg-tertiary);
    border-radius: 10px;
    overflow: hidden;
    margin-bottom: 1rem;
}

.progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--success-color), var(--info-color));
    border-radius: 10px;
    transition: width 0.3s ease;
    width: 0%;
}

.epoch-stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 1rem;
}

.stat-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem;
    background-color: var(--bg-tertiary);
    border-radius: 4px;
}

.stat-item .label {
    color: var(--text-secondary);
    font-size: 0.875rem;
}

.stat-item .value {
    font-weight: bold;
    color: var(--text-primary);
}

.stat-item .value.grade {
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.875rem;
}

.grade-a-plus, .grade-a { background-color: var(--success-color); color: white; }
.grade-b-plus, .grade-b { background-color: var(--info-color); color: white; }
.grade-c-plus, .grade-c { background-color: var(--warning-color); color: black; }
.grade-d, .grade-f { background-color: var(--error-color); color: white; }

.history-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
}

.history-table th,
.history-table td {
    padding: 0.75rem;
    text-align: left;
    border-bottom: 1px solid var(--border-color);
}

.history-table th {
    background-color: var(--bg-tertiary);
    font-weight: 600;
    color: var(--text-primary);
}

.history-table td {
    color: var(--text-secondary);
}

/* Utility Classes */
.text-success { color: var(--success-color); }
.text-warning { color: var(--warning-color); }
.text-error { color: var(--error-color); }
.text-info { color: var(--info-color); }
`
}

func (w *WebUI) getMainJS() string {
	return `
// Solana Exporter Dashboard JavaScript
class SolanaDashboard {
    constructor() {
        this.updateInterval = 15000; // 15 seconds
        this.isLoading = false;
        this.init();
    }

    init() {
        this.startAutoRefresh();
        this.setupEventListeners();
        this.loadInitialData();
    }

    setupEventListeners() {
        const refreshBtn = document.getElementById('refresh-batches');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => this.loadVoteBatches());
        }

        const refreshEpochBtn = document.getElementById('refresh-epoch');
        if (refreshEpochBtn) {
            refreshEpochBtn.addEventListener('click', () => this.loadTVCHistory());
        }
    }

    startAutoRefresh() {
        // Refresh dashboard data every 15 seconds
        setInterval(() => {
            if (!this.isLoading) {
                this.loadDashboardData();
            }
        }, this.updateInterval);
        
        // Refresh vote batches more frequently (every 5 seconds) for live updates
        setInterval(() => {
            this.loadVoteBatches();
        }, 5000);
    }

    async loadInitialData() {
        await Promise.all([
            this.loadDashboardData(),
            this.loadVoteBatches(),
            this.loadTVCHistory()
        ]);
    }

    async loadDashboardData() {
        if (this.isLoading) return;
        this.isLoading = true;

        try {
            const response = await fetch('/api/dashboard');
            const data = await response.json();

            this.updateDashboard(data);
            this.updateLastUpdateTime();
        } catch (error) {
            console.error('Failed to load dashboard data:', error);
            this.showError('Failed to load dashboard data');
        } finally {
            this.isLoading = false;
        }
    }

    async loadVoteBatches() {
        try {
            const response = await fetch('/api/vote-batches');
            const batches = await response.json();

            this.updateBatchTable(batches);
        } catch (error) {
            console.error('Failed to load vote batches:', error);
            this.showBatchError('Failed to load batch data');
        }
    }

    updateDashboard(data) {
        // Update node health
        this.updateElement('health-status', data.node_health.is_healthy ? 'Healthy' : 'Unhealthy');
        this.updateElement('node-health-value', data.node_health.is_healthy ? 'Healthy' : 'Unhealthy');
        this.updateElement('slots-behind', data.node_health.slots_behind + ' slots behind');

        this.updateStatusIndicator('health-indicator-dot', data.node_health.is_healthy);
        this.updateCard('node-health-card', data.node_health.is_healthy ? 'healthy' : 'error');

        // Update validator status
        const validatorStatus = data.validator_status.is_delinquent ? 'Delinquent' : 'Active';
        this.updateElement('validator-status-value', validatorStatus);
        this.updateElement('validator-stake', this.formatNumber(data.validator_status.active_stake) + ' SOL');

        this.updateStatusIndicator('validator-indicator-dot', !data.validator_status.is_delinquent);
        this.updateCard('validator-status-card', data.validator_status.is_delinquent ? 'error' : 'healthy');

        // Update vote performance
        const performance = 100 - data.vote_credits.missed_percent;
        this.updateElement('vote-performance-value', performance.toFixed(1) + '%');
        this.updateElement('vote-lag', data.vote_credits.vote_lag + ' slots lag');

        this.updateStatusIndicator('vote-indicator-dot', performance >= 95);
        this.updateCard('vote-performance-card', this.getPerformanceStatus(performance));

        // Update epoch info
        this.updateElement('current-epoch-value', data.network_activity.current_epoch);
        this.updateElement('epoch-progress', data.network_activity.epoch_progress.toFixed(1) + '% complete');

        // Update network activity
        this.updateElement('current-slot', this.formatNumber(data.network_activity.current_slot));
        this.updateElement('block-height', this.formatNumber(data.network_activity.block_height));
        this.updateElement('transactions-total', this.formatNumber(data.network_activity.transactions_total));
    }

    updateBatchTable(batches) {
        const tbody = document.getElementById('batch-table-body');
        if (!tbody) return;

        if (!batches || batches.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="loading-cell">No batch data available</td></tr>';
            return;
        }

        const rows = batches.slice(0, 50).map((batch, index) => {
            const performanceClass = this.getPerformanceClass(batch.performance);
            const isLive = batch.is_live || false;
            const liveIndicator = isLive ? ' <span class="live-indicator">● LIVE</span>' : '';
            const rowClass = isLive ? 'live-batch-row' : '';
            
            return '<tr class="' + rowClass + '">' +
                '<td>' + batch.batch_id + liveIndicator + '</td>' +
                '<td>' + batch.start_time + '</td>' +
                '<td><code>' + batch.slot_range + '</code></td>' +
                '<td>' + batch.avg_latency.toFixed(2) + 's</td>' +
                '<td class="' + (batch.missed_tvcs > 1000 ? 'text-warning' : '') + '">' + this.formatNumber(batch.missed_tvcs) + '</td>' +
                '<td class="' + (batch.missed_slots > 0 ? 'text-warning' : '') + '">' + batch.missed_slots + '</td>' +
                '<td class="' + performanceClass + '">' + batch.performance.toFixed(1) + '%</td>' +
                '</tr>';
        }).join('');

        tbody.innerHTML = rows;
    }

    async loadTVCHistory() {
        try {
            const response = await fetch('/api/epoch-summary');
            const data = await response.json();

            this.updateEpochProgress(data.current_epoch);
            this.updateHistoryTable(data.historical_epochs);
        } catch (error) {
            console.error('Failed to load TVC history:', error);
        }
    }

    updateEpochProgress(currentEpoch) {
        if (!currentEpoch) return;

        // Update progress bar
        const progressBar = document.getElementById('epoch-progress-bar');
        if (progressBar) {
            const progress = (currentEpoch.history_points.length > 0) ?
                currentEpoch.history_points[currentEpoch.history_points.length - 1].epoch_progress : 0;
            progressBar.style.width = progress + '%';
        }

        // Update stats
        this.updateElement('current-performance', currentEpoch.performance_percent.toFixed(1) + '%');
        this.updateElement('current-grade', currentEpoch.grade || '--');
        this.updateElement('current-missed', this.formatNumber(currentEpoch.missed_credits));

        // Update grade styling
        const gradeElement = document.getElementById('current-grade');
        if (gradeElement && currentEpoch.grade) {
            gradeElement.className = 'value grade grade-' + currentEpoch.grade.toLowerCase().replace('+', '-plus');
        }
    }

    updateHistoryTable(historicalEpochs) {
        const tbody = document.getElementById('history-table-body');
        if (!tbody) return;

        if (!historicalEpochs || historicalEpochs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="loading-cell">No historical data available</td></tr>';
            return;
        }

        const rows = historicalEpochs.slice(0, 5).map(epoch => {
            const duration = this.formatDuration(epoch.duration);
            const gradeClass = 'grade-' + epoch.grade.toLowerCase().replace('+', '-plus');

            return '<tr>' +
                '<td>' + epoch.epoch + '</td>' +
                '<td class="' + this.getPerformanceClass(epoch.performance_percent) + '">' + epoch.performance_percent.toFixed(1) + '%</td>' +
                '<td><span class="grade ' + gradeClass + '">' + epoch.grade + '</span></td>' +
                '<td>' + this.formatNumber(epoch.missed_credits) + '</td>' +
                '<td>' + duration + '</td>' +
                '</tr>';
        }).join('');

        tbody.innerHTML = rows;
    }

    formatDuration(durationNs) {
        // Convert nanoseconds to hours
        const hours = Math.round(durationNs / 1000000000 / 3600 * 10) / 10;
        if (hours < 1) {
            const minutes = Math.round(durationNs / 1000000000 / 60);
            return minutes + 'm';
        }
        return hours + 'h';
    }

    updateElement(id, value) {
        const element = document.getElementById(id);
        if (element) {
            element.textContent = value;
        }
    }

    updateStatusIndicator(id, isHealthy) {
        const indicator = document.getElementById(id);
        if (indicator) {
            indicator.className = 'status-indicator ' + (isHealthy ? 'healthy' : 'error');
        }
    }

    updateCard(cardId, status) {
        const card = document.getElementById(cardId);
        if (card) {
            card.className = 'card status-card ' + status;
        }
    }

    getPerformanceStatus(performance) {
        if (performance >= 98) return 'healthy';
        if (performance >= 90) return 'warning';
        return 'error';
    }

    getPerformanceClass(performance) {
        if (performance >= 95) return 'performance-good';
        if (performance >= 90) return 'performance-warning';
        return 'performance-error';
    }

    formatNumber(num) {
        if (num >= 1000000000) {
            return (num / 1000000000).toFixed(1) + 'B';
        }
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        }
        if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toString();
    }

    updateLastUpdateTime() {
        const now = new Date();
        const timeStr = now.toLocaleTimeString();
        this.updateElement('last-update', timeStr);
    }

    showError(message) {
        console.error(message);
        // Could add toast notifications here
    }

    showBatchError(message) {
        const tbody = document.getElementById('batch-table-body');
        if (tbody) {
            tbody.innerHTML = '<tr><td colspan="7" class="loading-cell text-error">' + message + '</td></tr>';
        }
    }
}

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new SolanaDashboard();
});

// Add some utility functions
window.copyToClipboard = function(text) {
    navigator.clipboard.writeText(text).then(() => {
        console.log('Copied to clipboard:', text);
    });
};

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new SolanaDashboard();
});
`
}
