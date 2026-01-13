package main

import (
	"context"
	"net/http"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	slog.Init()
	logger := slog.Get()
	ctx := context.Background()

	config, err := NewExporterConfigFromCLI(ctx)
	if err != nil {
		logger.Fatal(err)
	}
	if config.ComprehensiveSlotTracking {
		logger.Warn(
			"Comprehensive slot tracking will lead to potentially thousands of new " +
				"Prometheus metrics being created every epoch.",
		)
	}

	rpcClient := rpc.NewRPCClient(config.RpcUrl, config.HttpTimeout)
	var referenceRpcClient *rpc.Client
	if config.ReferenceRpcUrl != "" {
		referenceRpcClient = rpc.NewRPCClient(config.ReferenceRpcUrl, config.HttpTimeout)
	}
	// Create shared vote account cache to reduce RPC calls
	voteAccountCache := NewVoteAccountCache(rpcClient)
	collector := NewSolanaCollector(rpcClient, referenceRpcClient, config, voteAccountCache)
	slotWatcher := NewSlotWatcher(rpcClient, config)
	voteBatchAnalyzer := NewVoteBatchAnalyzer(rpcClient, config, voteAccountCache)
	tvcHistoryManager := NewTVCHistoryManager(rpcClient, voteBatchAnalyzer, config, voteAccountCache)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go slotWatcher.WatchSlots(ctx)
	go tvcHistoryManager.StartHistoryCollection(ctx)

	prometheus.MustRegister(collector)
	prometheus.MustRegister(tvcHistoryManager)

	// Set up HTTP routes
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Web UI setup
	webUI := NewWebUI(collector, voteBatchAnalyzer, tvcHistoryManager, rpcClient, config)
	webUI.RegisterHandlers(mux)

	logger.Infof("listening on %s", config.ListenAddress)
	logger.Infof("Web UI available at http://%s", config.ListenAddress)
	logger.Infof("Prometheus metrics at http://%s/metrics", config.ListenAddress)
	logger.Fatal(http.ListenAndServe(config.ListenAddress, mux))
}
