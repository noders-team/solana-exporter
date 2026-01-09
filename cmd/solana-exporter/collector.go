package main

import (
	"context"
	"fmt"
	"strconv"

	"slices"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	SkipStatusLabel      = "status"
	StateLabel           = "state"
	NodekeyLabel         = "nodekey"
	VotekeyLabel         = "votekey"
	VersionLabel         = "version"
	IdentityLabel        = "identity"
	AddressLabel         = "address"
	EpochLabel           = "epoch"
	TransactionTypeLabel = "transaction_type"
	CurrentVersionLabel  = "current"
	LatestVersionLabel   = "latest"

	StatusSkipped = "skipped"
	StatusValid   = "valid"

	StateCurrent    = "current"
	StateDelinquent = "delinquent"

	TransactionTypeVote    = "vote"
	TransactionTypeNonVote = "non_vote"
)

type SolanaCollector struct {
	rpcClient          *rpc.Client
	referenceRpcClient *rpc.Client
	logger             *zap.SugaredLogger

	config *ExporterConfig

	/// descriptors:
	ValidatorActiveStake    *GaugeDesc
	ClusterActiveStake      *GaugeDesc
	ValidatorLastVote       *GaugeDesc
	ClusterLastVote         *GaugeDesc
	ValidatorRootSlot       *GaugeDesc
	ClusterRootSlot         *GaugeDesc
	ValidatorDelinquent     *GaugeDesc
	ClusterValidatorCount   *GaugeDesc
	ValidatorCommission      *GaugeDesc
	ValidatorEpochCredits    *GaugeDesc
	ValidatorExpectedCredits *GaugeDesc
	ValidatorCreditsMissed   *GaugeDesc
	AccountBalances          *GaugeDesc

	// Vote batch analyzer
	voteBatchAnalyzer        *VoteBatchAnalyzer
	NodeVersion              *GaugeDesc
	NodeIdentity             *GaugeDesc
	NodeIsHealthy            *GaugeDesc
	NodeNumSlotsBehind       *GaugeDesc
	NodeMinimumLedgerSlot    *GaugeDesc
	NodeFirstAvailableBlock  *GaugeDesc
	NodeIsActive             *GaugeDesc
	NodeVersionOutdated      *GaugeDesc
}

func NewSolanaCollector(rpcClient *rpc.Client, referenceRpcClient *rpc.Client, config *ExporterConfig) *SolanaCollector {
	collector := &SolanaCollector{
		rpcClient:          rpcClient,
		referenceRpcClient: referenceRpcClient,
		logger:             slog.Get(),
		config:             config,
		ValidatorActiveStake: NewGaugeDesc(
			"solana_validator_active_stake",
			fmt.Sprintf("Active stake (in SOL) per validator (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterActiveStake: NewGaugeDesc(
			"solana_cluster_active_stake",
			"Total active stake (in SOL) of the cluster",
		),
		ValidatorLastVote: NewGaugeDesc(
			"solana_validator_last_vote",
			fmt.Sprintf("Last voted-on slot per validator (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterLastVote: NewGaugeDesc(
			"solana_cluster_last_vote",
			"Most recent voted-on slot of the cluster",
		),
		ValidatorRootSlot: NewGaugeDesc(
			"solana_validator_root_slot",
			fmt.Sprintf("Root slot per validator (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterRootSlot: NewGaugeDesc(
			"solana_cluster_root_slot",
			"Max root slot of the cluster",
		),
		ValidatorDelinquent: NewGaugeDesc(
			"solana_validator_delinquent",
			fmt.Sprintf("Whether a validator (represented by %s and %s) is delinquent", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterValidatorCount: NewGaugeDesc(
			"solana_cluster_validator_count",
			fmt.Sprintf(
				"Total number of validators in the cluster, grouped by %s ('%s' or '%s')",
				StateLabel, StateCurrent, StateDelinquent,
			),
			StateLabel,
		),
		AccountBalances: NewGaugeDesc(
			"solana_account_balance",
			fmt.Sprintf("Solana account balances, grouped by %s", AddressLabel),
			AddressLabel,
		),
		NodeVersion: NewGaugeDesc(
			"solana_node_version",
			"Node version of solana",
			VersionLabel,
		),
		NodeIdentity: NewGaugeDesc(
			"solana_node_identity",
			"Node identity of solana",
			IdentityLabel,
		),
		NodeIsHealthy: NewGaugeDesc(
			"solana_node_is_healthy",
			"Whether the node is healthy",
		),
		NodeNumSlotsBehind: NewGaugeDesc(
			"solana_node_num_slots_behind",
			"The number of slots that the node is behind the latest cluster confirmed slot.",
		),
		NodeMinimumLedgerSlot: NewGaugeDesc(
			"solana_node_minimum_ledger_slot",
			"The lowest slot that the node has information about in its ledger.",
		),
		NodeFirstAvailableBlock: NewGaugeDesc(
			"solana_node_first_available_block",
			"The slot of the lowest confirmed block that has not been purged from the node's ledger.",
		),
		NodeIsActive: NewGaugeDesc(
			"solana_node_is_active",
			fmt.Sprintf("Whether the node is active and participating in consensus (using %s pubkey)", IdentityLabel),
			IdentityLabel,
		),
		ValidatorCommission: NewGaugeDesc(
			"solana_validator_commission",
			fmt.Sprintf("Validator commission, as a percentage (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ValidatorEpochCredits: NewGaugeDesc(
			"solana_validator_epoch_credits",
			fmt.Sprintf("Vote credits earned by validator in current epoch (represented by %s, %s, and %s)", VotekeyLabel, NodekeyLabel, EpochLabel),
			VotekeyLabel, NodekeyLabel, EpochLabel,
		),
		ValidatorExpectedCredits: NewGaugeDesc(
			"solana_validator_expected_credits",
			fmt.Sprintf("Expected vote credits for validator in current epoch (represented by %s, %s, and %s)", VotekeyLabel, NodekeyLabel, EpochLabel),
			VotekeyLabel, NodekeyLabel, EpochLabel,
		),
		ValidatorCreditsMissed: NewGaugeDesc(
			"solana_validator_credits_missed_pct",
			fmt.Sprintf("Percentage of vote credits missed by validator in current epoch (represented by %s, %s, and %s)", VotekeyLabel, NodekeyLabel, EpochLabel),
			VotekeyLabel, NodekeyLabel, EpochLabel,
		),
		NodeVersionOutdated: NewGaugeDesc(
			"solana_node_version_outdated",
			fmt.Sprintf("Whether the node version is outdated compared to reference RPC (1=outdated, 0=up-to-date), with %s and %s labels", CurrentVersionLabel, LatestVersionLabel),
			CurrentVersionLabel, LatestVersionLabel,
		),
		voteBatchAnalyzer: NewVoteBatchAnalyzer(rpcClient, config),
	}
	return collector
}

func (c *SolanaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.NodeVersion.Desc
	ch <- c.NodeIdentity.Desc
	ch <- c.ValidatorActiveStake.Desc
	ch <- c.ClusterActiveStake.Desc
	ch <- c.ValidatorLastVote.Desc
	ch <- c.ClusterLastVote.Desc
	ch <- c.ValidatorRootSlot.Desc
	ch <- c.ClusterRootSlot.Desc
	ch <- c.ValidatorDelinquent.Desc
	ch <- c.ClusterValidatorCount.Desc
	ch <- c.ValidatorCommission.Desc
	ch <- c.ValidatorEpochCredits.Desc
	ch <- c.ValidatorExpectedCredits.Desc
	ch <- c.ValidatorCreditsMissed.Desc
	ch <- c.AccountBalances.Desc

	// Vote batch analyzer descriptors
	c.voteBatchAnalyzer.Describe(ch)
	ch <- c.NodeIsHealthy.Desc
	ch <- c.NodeNumSlotsBehind.Desc
	ch <- c.NodeMinimumLedgerSlot.Desc
	ch <- c.NodeFirstAvailableBlock.Desc
	ch <- c.NodeIsActive.Desc
	ch <- c.ValidatorCommission.Desc
	ch <- c.NodeVersionOutdated.Desc
}

func (c *SolanaCollector) collectVoteAccounts(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.config.LightMode {
		c.logger.Debug("Skipping vote-accounts collection in light mode.")
		return
	}
	c.logger.Info("Collecting vote accounts...")
	voteAccounts, err := c.rpcClient.GetVoteAccounts(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		c.logger.Errorf("failed to get vote accounts: %v", err)
		ch <- c.ValidatorActiveStake.NewInvalidMetric(err)
		ch <- c.ClusterActiveStake.NewInvalidMetric(err)
		ch <- c.ValidatorLastVote.NewInvalidMetric(err)
		ch <- c.ClusterLastVote.NewInvalidMetric(err)
		ch <- c.ValidatorRootSlot.NewInvalidMetric(err)
		ch <- c.ClusterRootSlot.NewInvalidMetric(err)
		ch <- c.ValidatorDelinquent.NewInvalidMetric(err)
		ch <- c.ClusterValidatorCount.NewInvalidMetric(err)
		ch <- c.ValidatorCommission.NewInvalidMetric(err)
		return
	}

	var (
		totalStake  float64
		maxLastVote float64
		maxRootSlot float64
	)

	// Get current epoch info for credits calculation
	epochInfo, err := c.rpcClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		c.logger.Errorf("failed to get epoch info for credits calculation: %v", err)
	}
	for _, account := range append(voteAccounts.Current, voteAccounts.Delinquent...) {
		accounts := []string{account.VotePubkey, account.NodePubkey}
		stake, lastVote, rootSlot, commission :=
			float64(account.ActivatedStake)/rpc.LamportsInSol,
			float64(account.LastVote),
			float64(account.RootSlot),
			float64(account.Commission)

		if slices.Contains(c.config.Nodekeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
			ch <- c.ValidatorActiveStake.MustNewConstMetric(stake, accounts...)
			ch <- c.ValidatorLastVote.MustNewConstMetric(lastVote, accounts...)
			ch <- c.ValidatorRootSlot.MustNewConstMetric(rootSlot, accounts...)
			ch <- c.ValidatorCommission.MustNewConstMetric(commission, accounts...)

			// Collect vote credits metrics if epoch info is available
			if epochInfo != nil {
				c.collectVoteCreditsMetrics(ctx, ch, account, epochInfo, accounts...)
			}

			// Collect vote batch metrics for detailed TVC analysis
			c.voteBatchAnalyzer.CollectVoteBatchMetrics(ctx, ch, account.NodePubkey, account.VotePubkey)
		}

		totalStake += stake
		maxLastVote = max(maxLastVote, lastVote)
		maxRootSlot = max(maxRootSlot, rootSlot)
	}

	{
		for _, account := range voteAccounts.Current {
			if slices.Contains(c.config.Nodekeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
				ch <- c.ValidatorDelinquent.MustNewConstMetric(0, account.VotePubkey, account.NodePubkey)
			}
		}
		for _, account := range voteAccounts.Delinquent {
			if slices.Contains(c.config.Nodekeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
				ch <- c.ValidatorDelinquent.MustNewConstMetric(1, account.VotePubkey, account.NodePubkey)
			}
		}
	}

	ch <- c.ClusterActiveStake.MustNewConstMetric(totalStake)
	ch <- c.ClusterLastVote.MustNewConstMetric(maxLastVote)
	ch <- c.ClusterRootSlot.MustNewConstMetric(maxRootSlot)
	ch <- c.ClusterValidatorCount.MustNewConstMetric(float64(len(voteAccounts.Current)), StateCurrent)
	ch <- c.ClusterValidatorCount.MustNewConstMetric(float64(len(voteAccounts.Delinquent)), StateDelinquent)

	c.logger.Info("Vote accounts collected.")
}

// collectVoteCreditsMetrics collects vote credits related metrics for a validator
func (c *SolanaCollector) collectVoteCreditsMetrics(
	ctx context.Context,
	ch chan<- prometheus.Metric,
	account rpc.VoteAccount,
	epochInfo *rpc.EpochInfo,
	labelValues ...string,
) {
	// Get detailed vote account data to access epoch credits
	var voteAccountData rpc.VoteAccountData
	_, err := rpc.GetAccountInfo(ctx, c.rpcClient, rpc.CommitmentFinalized, account.VotePubkey, &voteAccountData)
	if err != nil {
		c.logger.Errorf("failed to get vote account info for %s: %v", account.VotePubkey, err)
		return
	}

	// Find credits for current epoch
	var currentEpochCredits, previousEpochCredits int64
	for _, credit := range voteAccountData.EpochCredits {
		if credit.Epoch == epochInfo.Epoch {
			// Convert string credits to int64
			if credits, err := strconv.ParseInt(credit.Credits, 10, 64); err == nil {
				currentEpochCredits = credits
			}
			if prevCredits, err := strconv.ParseInt(credit.PreviousCredits, 10, 64); err == nil {
				previousEpochCredits = prevCredits
			}
			break
		}
	}

	// Calculate credits earned this epoch
	epochCreditsEarned := currentEpochCredits - previousEpochCredits

	// Calculate expected credits based on slot progress in epoch
	slotsInCurrentEpoch := epochInfo.SlotIndex + 1 // +1 because SlotIndex is 0-based
	expectedCredits := slotsInCurrentEpoch // Ideally, 1 credit per slot

	// Calculate percentage of credits missed
	var creditsMissedPct float64
	if expectedCredits > 0 {
		creditsMissedPct = float64(expectedCredits-epochCreditsEarned) / float64(expectedCredits) * 100.0
		if creditsMissedPct < 0 {
			creditsMissedPct = 0 // Can't miss negative credits
		}
	}

	// Emit metrics with epoch label
	labelsWithEpoch := append(labelValues, fmt.Sprintf("%d", epochInfo.Epoch))
	ch <- c.ValidatorEpochCredits.MustNewConstMetric(float64(epochCreditsEarned), labelsWithEpoch...)
	ch <- c.ValidatorExpectedCredits.MustNewConstMetric(float64(expectedCredits), labelsWithEpoch...)
	ch <- c.ValidatorCreditsMissed.MustNewConstMetric(creditsMissedPct, labelsWithEpoch...)
}

func (c *SolanaCollector) collectVersion(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting version...")
	version, err := c.rpcClient.GetVersion(ctx)
	if err != nil {
		c.logger.Errorf("failed to get version: %v", err)
		ch <- c.NodeVersion.NewInvalidMetric(err)
		return
	}

	ch <- c.NodeVersion.MustNewConstMetric(1, version)
	c.logger.Info("Version collected.")

	// Сравнение с reference RPC версией
	if c.referenceRpcClient != nil {
		c.collectVersionComparison(ctx, ch, version)
	}
}

func (c *SolanaCollector) collectVersionComparison(ctx context.Context, ch chan<- prometheus.Metric, currentVersion string) {
	c.logger.Info("Collecting version comparison...")

	referenceVersion, err := c.referenceRpcClient.GetVersion(ctx)
	if err != nil {
		c.logger.Errorf("failed to get reference version: %v", err)
		ch <- c.NodeVersionOutdated.NewInvalidMetric(err)
		return
	}

	currentSemVer, err := ParseSemVer(currentVersion)
	if err != nil {
		c.logger.Errorf("failed to parse current version '%s': %v", currentVersion, err)
		ch <- c.NodeVersionOutdated.NewInvalidMetric(err)
		return
	}

	referenceSemVer, err := ParseSemVer(referenceVersion)
	if err != nil {
		c.logger.Errorf("failed to parse reference version '%s': %v", referenceVersion, err)
		ch <- c.NodeVersionOutdated.NewInvalidMetric(err)
		return
	}

	isOutdated := BoolToFloat64(currentSemVer.IsOlderThan(referenceSemVer))
	ch <- c.NodeVersionOutdated.MustNewConstMetric(isOutdated, currentVersion, referenceVersion)

	c.logger.Infof("Version comparison: current=%s, reference=%s, outdated=%v", currentVersion, referenceVersion, isOutdated == 1)
}

func (c *SolanaCollector) collectIdentity(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting identity...")
	identity, err := c.rpcClient.GetIdentity(ctx)
	if err != nil {
		c.logger.Errorf("failed to get identity: %v", err)
		ch <- c.NodeIdentity.NewInvalidMetric(err)
		return
	}

	if c.config.ActiveIdentity != "" {
		isActive := 0
		if c.config.ActiveIdentity == identity {
			isActive = 1
		}
		ch <- c.NodeIsActive.MustNewConstMetric(float64(isActive), identity)
		c.logger.Info("NodeIsActive collected.")
	}

	ch <- c.NodeIdentity.MustNewConstMetric(1, identity)
	c.logger.Info("Identity collected.")
}

func (c *SolanaCollector) collectMinimumLedgerSlot(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.config.LightMode {
		c.logger.Debug("Skipping minimum ledger slot collection in light mode.")
		return
	}
	c.logger.Info("Collecting minimum ledger slot...")
	slot, err := c.rpcClient.GetMinimumLedgerSlot(ctx)
	if err != nil {
		c.logger.Errorf("failed to get minimum lidger slot: %v", err)
		ch <- c.NodeMinimumLedgerSlot.NewInvalidMetric(err)
		return
	}

	ch <- c.NodeMinimumLedgerSlot.MustNewConstMetric(float64(slot))
	c.logger.Info("Minimum ledger slot collected.")
}

func (c *SolanaCollector) collectFirstAvailableBlock(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.config.LightMode {
		c.logger.Debug("Skipping first available block collection in light mode.")
		return
	}
	c.logger.Info("Collecting first available block...")
	block, err := c.rpcClient.GetFirstAvailableBlock(ctx)
	if err != nil {
		c.logger.Errorf("failed to get first available block: %v", err)
		ch <- c.NodeFirstAvailableBlock.NewInvalidMetric(err)
		return
	}

	ch <- c.NodeFirstAvailableBlock.MustNewConstMetric(float64(block))
	c.logger.Info("First available block collected.")
}

func (c *SolanaCollector) collectBalances(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.config.LightMode {
		c.logger.Debug("Skipping balance collection in light mode.")
		return
	}
	c.logger.Info("Collecting balances...")
	balances, err := FetchBalances(
		ctx, c.rpcClient, CombineUnique(c.config.BalanceAddresses, c.config.Nodekeys, c.config.Votekeys),
	)
	if err != nil {
		c.logger.Errorf("failed to get balances: %v", err)
		ch <- c.AccountBalances.NewInvalidMetric(err)
		return
	}

	for address, balance := range balances {
		ch <- c.AccountBalances.MustNewConstMetric(balance, address)
	}
	c.logger.Info("Balances collected.")
}

func (c *SolanaCollector) collectHealth(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting health...")

	health, err := c.rpcClient.GetHealth(ctx)
	isHealthy, isHealthyErr, numSlotsBehind, numSlotsBehindErr := ExtractHealthAndNumSlotsBehind(health, err)
	if isHealthyErr != nil {
		c.logger.Errorf("failed to determine node health: %v", isHealthyErr)
		ch <- c.NodeIsHealthy.NewInvalidMetric(err)
	} else {
		ch <- c.NodeIsHealthy.MustNewConstMetric(BoolToFloat64(isHealthy))
	}

	if numSlotsBehindErr != nil {
		c.logger.Errorf("failed to determine number of slots behind: %v", numSlotsBehindErr)
		ch <- c.NodeNumSlotsBehind.NewInvalidMetric(numSlotsBehindErr)
	} else {
		ch <- c.NodeNumSlotsBehind.MustNewConstMetric(float64(numSlotsBehind))
	}

	c.logger.Info("Health collected.")
	return
}

func (c *SolanaCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Info("========== BEGIN COLLECTION ==========")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.collectHealth(ctx, ch)
	c.collectMinimumLedgerSlot(ctx, ch)
	c.collectFirstAvailableBlock(ctx, ch)
	c.collectVoteAccounts(ctx, ch)
	c.collectVersion(ctx, ch)
	c.collectIdentity(ctx, ch)
	c.collectBalances(ctx, ch)

	// Collect vote batch metrics
	c.voteBatchAnalyzer.Collect(ch)

	c.logger.Info("=========== END COLLECTION ===========")
}
