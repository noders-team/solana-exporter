# Solana Exporter - Complete Metrics Reference

This document provides a comprehensive overview of all metrics exported by Solana Exporter, including the enhanced Vote Credits monitoring capabilities.

## 📊 Metrics Categories

### 🏛️ Validator Performance Metrics

#### Stake & Status
- **`solana_validator_active_stake`** *(Gauge)*
  - **Description**: Active stake (in SOL) per validator
  - **Labels**: `votekey`, `nodekey`
  - **Use Case**: Monitor validator stake changes, track delegation

- **`solana_validator_delinquent`** *(Gauge)*
  - **Description**: Whether validator is delinquent (0=active, 1=delinquent)
  - **Labels**: `votekey`, `nodekey`
  - **Use Case**: Alert on validator going offline/delinquent

- **`solana_validator_commission`** *(Gauge)*
  - **Description**: Validator commission as percentage
  - **Labels**: `votekey`, `nodekey`
  - **Use Case**: Track commission changes, compare rates

#### Voting Performance
- **`solana_validator_last_vote`** *(Gauge)*
  - **Description**: Last voted-on slot per validator
  - **Labels**: `votekey`, `nodekey`
  - **Use Case**: Detect voting lag, monitor participation

- **`solana_validator_root_slot`** *(Gauge)*
  - **Description**: Root slot per validator (confirmed/finalized)
  - **Labels**: `votekey`, `nodekey`
  - **Use Case**: Monitor consensus participation

#### 🆕 Vote Credits (Enhanced TVC Monitoring)
- **`solana_validator_epoch_credits`** *(Gauge)*
  - **Description**: Vote credits earned by validator in current epoch
  - **Labels**: `votekey`, `nodekey`, `epoch`
  - **Use Case**: Track actual voting performance

- **`solana_validator_expected_credits`** *(Gauge)*
  - **Description**: Expected vote credits based on epoch progress
  - **Labels**: `votekey`, `nodekey`, `epoch`
  - **Use Case**: Calculate performance baseline

- **`solana_validator_credits_missed_pct`** *(Gauge)*
  - **Description**: Percentage of vote credits missed by validator
  - **Labels**: `votekey`, `nodekey`, `epoch`
  - **Use Case**: **PRIMARY TVC MONITORING METRIC** - alert on poor voting performance

#### 🆕 Vote Batch Analysis (Granular TVC Tracking)
- **`solana_vote_batch_missed_tvcs`** *(Gauge)*
  - **Description**: Number of missed TVCs per vote batch
  - **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
  - **Use Case**: Identify specific batches with high TVC misses

- **`solana_vote_batch_missed_slots`** *(Gauge)*
  - **Description**: Number of missed slots per vote batch
  - **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
  - **Use Case**: Track slot-level voting gaps

- **`solana_vote_batch_avg_latency_seconds`** *(Gauge)*
  - **Description**: Average voting latency per batch in seconds
  - **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
  - **Use Case**: Monitor voting response times and network performance

- **`solana_vote_batch_performance_pct`** *(Gauge)*
  - **Description**: Vote performance percentage per batch
  - **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
  - **Use Case**: **GRANULAR BATCH PERFORMANCE METRIC** - track batch-level efficiency

- **`solana_vote_batch_slot_range`** *(Gauge)*
  - **Description**: Total number of slots covered by vote batch
  - **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
  - **Use Case**: Understand batch sizing and coverage

- **`solana_vote_batch_count_total`** *(Gauge)*
  - **Description**: Total number of vote batches analyzed
  - **Labels**: `nodekey`, `votekey`
  - **Use Case**: Track voting pattern consistency

#### Leader Performance
- **`solana_validator_leader_slots_total`** *(Counter)*
  - **Description**: Total leader slots processed by validator
  - **Labels**: `nodekey`, `status` (valid/skipped)
  - **Use Case**: Calculate skip rate, monitor block production

- **`solana_validator_leader_slots_by_epoch_total`** *(Counter)*
  - **Description**: Leader slots processed by epoch
  - **Labels**: `nodekey`, `status`, `epoch`
  - **Use Case**: Epoch-by-epoch performance analysis

- **`solana_validator_block_size`** *(Gauge)*
  - **Description**: Number of transactions per block produced
  - **Labels**: `nodekey`
  - **Use Case**: Monitor block utilization, MEV performance

#### Rewards
- **`solana_validator_inflation_rewards_total`** *(Counter)*
  - **Description**: Inflation rewards earned by validator
  - **Labels**: `votekey`, `epoch`
  - **Use Case**: Track staking rewards, revenue analysis

- **`solana_validator_fee_rewards_total`** *(Counter)*
  - **Description**: Transaction fee rewards earned
  - **Labels**: `nodekey`, `epoch`
  - **Use Case**: Track fee revenue, MEV earnings

### 🌐 Cluster-Wide Metrics

- **`solana_cluster_active_stake`** *(Gauge)*
  - **Description**: Total active stake of entire cluster
  - **Use Case**: Monitor network security, stake distribution

- **`solana_cluster_last_vote`** *(Gauge)*
  - **Description**: Most recent voted-on slot across cluster
  - **Use Case**: Network-wide voting health

- **`solana_cluster_root_slot`** *(Gauge)*
  - **Description**: Maximum root slot across cluster
  - **Use Case**: Consensus finality tracking

- **`solana_cluster_validator_count`** *(Gauge)*
  - **Description**: Total validator count by state
  - **Labels**: `state` (current/delinquent)
  - **Use Case**: Monitor network decentralization

- **`solana_cluster_slots_by_epoch_total`** *(Counter)*
  - **Description**: Total slots processed by cluster
  - **Labels**: `status`, `epoch`
  - **Use Case**: Network-wide performance metrics

### 🖥️ Node Health Metrics

#### Basic Health
- **`solana_node_is_healthy`** *(Gauge)*
  - **Description**: Whether the RPC node is healthy (0/1)
  - **Use Case**: **PRIMARY HEALTH CHECK** - alert if node unhealthy

- **`solana_node_num_slots_behind`** *(Gauge)*
  - **Description**: Number of slots node is behind cluster
  - **Use Case**: Monitor sync status, detect lag

- **`solana_node_is_active`** *(Gauge)*
  - **Description**: Whether node is active in consensus
  - **Labels**: `identity`
  - **Use Case**: Confirm validator participation

#### Node Information
- **`solana_node_version`** *(Gauge)*
  - **Description**: Current Solana version
  - **Labels**: `version`
  - **Use Case**: Track version upgrades

- **`solana_node_version_outdated`** *(Gauge)*
  - **Description**: Whether version is outdated (0/1)
  - **Labels**: `current`, `latest`
  - **Use Case**: Alert on outdated versions

- **`solana_node_identity`** *(Gauge)*
  - **Description**: Node identity pubkey
  - **Labels**: `identity`
  - **Use Case**: Node identification

#### Ledger Status
- **`solana_node_minimum_ledger_slot`** *(Gauge)*
  - **Description**: Lowest slot in node's ledger
  - **Use Case**: Monitor ledger retention

- **`solana_node_first_available_block`** *(Gauge)*
  - **Description**: First available block slot
  - **Use Case**: Track data availability

### 📈 Network Activity Metrics

#### Slot & Epoch Tracking
- **`solana_node_slot_height`** *(Gauge)*
  - **Description**: Current slot number
  - **Use Case**: **CORE METRIC** - network progress tracking

- **`solana_node_epoch_number`** *(Gauge)*
  - **Description**: Current epoch number
  - **Use Case**: Epoch transitions, rewards calculations

- **`solana_node_epoch_first_slot`** *(Gauge)*
  - **Description**: First slot of current epoch
  - **Use Case**: Epoch boundary calculations

- **`solana_node_epoch_last_slot`** *(Gauge)*
  - **Description**: Last slot of current epoch
  - **Use Case**: Epoch boundary calculations

#### Transaction Metrics
- **`solana_node_transactions_total`** *(Gauge)*
  - **Description**: Total transactions processed since genesis
  - **Use Case**: Network activity monitoring

- **`solana_node_block_height`** *(Gauge)*
  - **Description**: Current block height
  - **Use Case**: Block production tracking

### 💰 Balance Monitoring

- **`solana_account_balance`** *(Gauge)*
  - **Description**: SOL balance of monitored addresses
  - **Labels**: `address`
  - **Use Case**: Track treasury, operational wallets

## 🎯 Key Metrics for Different Use Cases

### 🚨 Critical Alerting (Must Monitor)
1. **`solana_validator_credits_missed_pct`** - Vote credit performance
2. **`solana_node_is_healthy`** - Node health status  
3. **`solana_validator_delinquent`** - Validator delinquency
4. **`solana_node_num_slots_behind`** - Sync status

### 📊 Performance Dashboard
1. **`solana_validator_active_stake`** - Stake tracking
2. **`solana_validator_leader_slots_total`** - Skip rate calculation
3. **`solana_validator_block_size`** - Block utilization
4. **`solana_validator_last_vote`** - Voting lag

### 💰 Revenue Tracking
1. **`solana_validator_inflation_rewards_total`** - Staking rewards
2. **`solana_validator_fee_rewards_total`** - Fee earnings
3. **`solana_validator_commission`** - Commission rates

### 🔧 Operational Monitoring
1. **`solana_node_version_outdated`** - Version management
2. **`solana_account_balance`** - Wallet monitoring
3. **`solana_node_minimum_ledger_slot`** - Storage management

## 📏 Metric Types Explained

- **Gauge**: Current value that can go up or down
- **Counter**: Monotonically increasing value (resets on restart)

## 🏷️ Label Reference

| Label | Description | Example Values |
|-------|-------------|----------------|
| `nodekey` | Validator identity pubkey | `"7Np41oeYqPeNDHae..."` |
| `votekey` | Validator vote account pubkey | `"84hoQQ9M4F8..."` |
| `epoch` | Epoch number | `"543"` |
| `status` | Slot processing status | `"valid"`, `"skipped"` |
| `state` | Validator state | `"current"`, `"delinquent"` |
| `address` | Account address | `"7Np41oeYqPeNDHae..."` |
| `version` | Software version | `"v1.18.22"` |
| `identity` | Node identity | `"7Np41oeYqPeNDHae..."` |

## 🚀 Quick Query Examples

### Vote Credits Performance
```promql
# Current vote performance percentage
100 - solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"}

# Vote lag in slots  
solana_node_slot_height - solana_validator_last_vote{nodekey="YOUR_NODEKEY"}
```

### Skip Rate Calculation
```promql
# Skip rate over last hour
rate(solana_validator_leader_slots_total{status="skipped"}[1h]) / 
rate(solana_validator_leader_slots_total[1h]) * 100
```

### Revenue Tracking
```promql
# Total rewards last 24h
increase(solana_validator_inflation_rewards_total[24h]) + 
increase(solana_validator_fee_rewards_total[24h])
```

## ⚡ Performance Impact

### Low Impact (Safe for all deployments)
- All gauge metrics
- Basic counters without comprehensive tracking

### Medium Impact (Monitor resource usage)
- `-comprehensive-vote-account-tracking` - Tracks all validators
- `-monitor-block-sizes` - Requires additional RPC calls

### High Impact (Use with caution)
- `-comprehensive-slot-tracking` - Creates thousands of metrics per epoch

## 🔗 Related Documentation

- [Vote Credits Monitoring Guide](vote-credits-monitoring-example.md)
- [Quick Start Guide](VOTE_CREDITS_QUICKSTART.md)
- [Prometheus Alerting Rules](prometheus/solana-vote-credits-rules.yml)
- [Solana RPC API Documentation](https://docs.solana.com/api)

---

**Total Metrics**: 32 unique metrics (9 new TVC monitoring metrics added)
- 3 Vote Credits metrics (epoch-level)
- 6 Vote Batch metrics (granular batch-level)
**Last Updated**: 2024 (Enhanced with comprehensive Vote Credits and Vote Batch tracking)