# Solana Exporter - Complete Metrics Catalog

> Comprehensive list of all 32+ metrics available in Solana Exporter, organized by categories for easy reference and UI development.

## 📊 Metrics Summary

**Total Metrics**: 32 unique metrics
**Categories**: 6 main categories
**Types**: Gauge (real-time values) and Counter (cumulative values)

---

## 🏛️ **1. VALIDATOR PERFORMANCE** (11 metrics)

### **Stake & Status**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_validator_active_stake` | Gauge | Active stake amount in SOL | `votekey`, `nodekey` | ⚡ |
| `solana_validator_delinquent` | Gauge | Delinquent status (0=active, 1=delinquent) | `votekey`, `nodekey` | 🚨 |
| `solana_validator_commission` | Gauge | Commission rate as percentage | `votekey`, `nodekey` | 💰 |

### **Vote Performance (Epoch Level)**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_validator_last_vote` | Gauge | Last voted slot number | `votekey`, `nodekey` | ⚡ |
| `solana_validator_root_slot` | Gauge | Root slot (finalized) | `votekey`, `nodekey` | ⚡ |
| `solana_validator_epoch_credits` | Gauge | Vote credits earned this epoch | `votekey`, `nodekey`, `epoch` | 📊 |
| `solana_validator_expected_credits` | Gauge | Expected vote credits this epoch | `votekey`, `nodekey`, `epoch` | 📊 |
| `solana_validator_credits_missed_pct` | Gauge | **PRIMARY TVC METRIC** - % missed votes | `votekey`, `nodekey`, `epoch` | 🚨 |

### **Block Production**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_validator_leader_slots_total` | Counter | Total leader slots processed | `nodekey`, `status` | 📈 |
| `solana_validator_leader_slots_by_epoch_total` | Counter | Leader slots by epoch | `nodekey`, `epoch`, `status` | 📈 |
| `solana_validator_block_size` | Gauge | Transactions per block | `nodekey`, `transaction_type` | 📊 |

---

## 🔍 **2. VOTE BATCH ANALYSIS** (6 metrics)

> **NEW FEATURE**: Granular vote batch monitoring for detailed TVC analysis

| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_vote_batch_missed_tvcs` | Gauge | **Missed TVCs per batch** | `nodekey`, `votekey`, `batch`, `slot_range` | 🚨 |
| `solana_vote_batch_missed_slots` | Gauge | Missed slots per batch | `nodekey`, `votekey`, `batch`, `slot_range` | ⚠️ |
| `solana_vote_batch_avg_latency_seconds` | Gauge | **Average voting latency** | `nodekey`, `votekey`, `batch`, `slot_range` | 🚨 |
| `solana_vote_batch_performance_pct` | Gauge | **Batch performance percentage** | `nodekey`, `votekey`, `batch`, `slot_range` | 🚨 |
| `solana_vote_batch_slot_range` | Gauge | Slots covered by batch | `nodekey`, `votekey`, `batch`, `slot_range` | 📊 |
| `solana_vote_batch_count_total` | Gauge | Total batches analyzed | `nodekey`, `votekey` | 📊 |

---

## 🌐 **3. CLUSTER METRICS** (4 metrics)

| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_cluster_active_stake` | Gauge | Total cluster active stake (SOL) | - | 📊 |
| `solana_cluster_last_vote` | Gauge | Most recent cluster vote slot | - | ⚡ |
| `solana_cluster_root_slot` | Gauge | Cluster max root slot | - | ⚡ |
| `solana_cluster_validator_count` | Gauge | Validator count by state | `state` | 📊 |
| `solana_cluster_slots_by_epoch_total` | Counter | Cluster slots by epoch | `epoch`, `status` | 📈 |

---

## 🖥️ **4. NODE HEALTH** (8 metrics)

### **Health Status**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_node_is_healthy` | Gauge | **Node health status** (0/1) | - | 🚨 |
| `solana_node_num_slots_behind` | Gauge | **Slots behind cluster** | - | 🚨 |
| `solana_node_is_active` | Gauge | Active in consensus (0/1) | `identity` | ⚠️ |

### **Node Information**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_node_version` | Gauge | Solana version number | `version` | ℹ️ |
| `solana_node_version_outdated` | Gauge | Version outdated flag (0/1) | `current`, `latest` | ⚠️ |
| `solana_node_identity` | Gauge | Node identity pubkey | `identity` | ℹ️ |

### **Ledger Status**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_node_minimum_ledger_slot` | Gauge | Lowest ledger slot | - | 📊 |
| `solana_node_first_available_block` | Gauge | First available block slot | - | 📊 |

---

## 📈 **5. NETWORK ACTIVITY** (6 metrics)

### **Slot & Epoch Tracking**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_node_slot_height` | Gauge | **Current slot number** | - | 🚨 |
| `solana_node_epoch_number` | Gauge | Current epoch number | - | ⚡ |
| `solana_node_epoch_first_slot` | Gauge | Current epoch first slot | - | 📊 |
| `solana_node_epoch_last_slot` | Gauge | Current epoch last slot | - | 📊 |

### **Transaction Activity**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_node_transactions_total` | Gauge | Total transactions since genesis | - | 📊 |
| `solana_node_block_height` | Gauge | Current block height | - | ⚡ |

---

## 💰 **6. FINANCIAL METRICS** (3 metrics)

### **Validator Rewards**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_validator_inflation_rewards_total` | Counter | **Staking rewards earned** | `votekey`, `epoch` | 💰 |
| `solana_validator_fee_rewards_total` | Counter | **Transaction fee earnings** | `nodekey`, `epoch` | 💰 |

### **Account Balances**
| Metric | Type | Description | Labels | Critical |
|--------|------|-------------|--------|----------|
| `solana_account_balance` | Gauge | SOL balance of monitored addresses | `address` | 💰 |

---

## 🎯 **CRITICAL METRICS FOR UI DASHBOARD**

### **🚨 RED ALERTS** (Must monitor continuously)
1. `solana_node_is_healthy` - Node health
2. `solana_validator_delinquent` - Validator status
3. `solana_validator_credits_missed_pct` - TVC performance
4. `solana_vote_batch_performance_pct` - Batch performance
5. `solana_node_num_slots_behind` - Sync status

### **⚠️ YELLOW WARNINGS** (Monitor trends)
1. `solana_vote_batch_avg_latency_seconds` - Voting latency
2. `solana_validator_last_vote` vs `solana_node_slot_height` - Vote lag
3. `solana_node_version_outdated` - Version status
4. `solana_validator_leader_slots_total` (skip rate calculation)

### **📊 KEY PERFORMANCE INDICATORS**
1. `solana_validator_active_stake` - Stake tracking
2. `solana_validator_inflation_rewards_total` - Revenue
3. `solana_validator_commission` - Commission rate
4. `solana_node_epoch_number` - Epoch progress

---

## 🏷️ **LABEL REFERENCE**

| Label | Description | Example Values |
|-------|-------------|----------------|
| `nodekey` | Validator identity pubkey | `"7Np41oeYqPeNDHae..."` |
| `votekey` | Validator vote account pubkey | `"84hoQQ9M4F8..."` |
| `batch` | Vote batch ID | `"1"`, `"2"`, `"3"` |
| `slot_range` | Slot range for batch | `"392257999-392259999"` |
| `epoch` | Epoch number | `"543"`, `"544"` |
| `status` | Processing status | `"valid"`, `"skipped"` |
| `state` | Validator state | `"current"`, `"delinquent"` |
| `address` | Account address | `"7Np41oeYqPeNDHae..."` |
| `version` | Software version | `"v1.18.22"` |
| `identity` | Node identity | `"7Np41oeYqPeNDHae..."` |
| `transaction_type` | Transaction type | `"vote"`, `"non_vote"` |

---

## 🎨 **UI DASHBOARD SECTIONS**

### **1. 🎯 Executive Summary (4 widgets)**
- Vote Credits Performance (primary KPI)
- Node Health Status
- Validator Status (active/delinquent)
- Current Epoch Progress

### **2. 📊 Vote Credits Detailed (6 widgets)**
- Epoch Credits vs Expected
- Vote Credits Miss Percentage
- Vote Batch Performance Table
- Batch Latency Chart
- Missed TVCs by Batch
- Vote Lag Real-time

### **3. 🏛️ Validator Performance (5 widgets)**
- Active Stake Tracking
- Skip Rate Chart
- Block Size Analysis
- Commission Rate
- Leader Slots Performance

### **4. 🌐 Network & Node Health (6 widgets)**
- Node Health Indicators
- Slots Behind Cluster
- Version Status
- Sync Progress
- Transaction Activity
- Epoch Timeline

### **5. 💰 Financial Dashboard (3 widgets)**
- Revenue Breakdown (inflation vs fees)
- Account Balance Monitoring
- Daily/Monthly Earnings

### **6. 🔍 Technical Details (4 widgets)**
- RPC Health
- Ledger Status
- Batch Analysis Details
- System Alerts Log

---

## 📋 **METRIC COLLECTION FREQUENCY**

| Category | Update Frequency | Resource Impact |
|----------|------------------|-----------------|
| **Node Health** | Every 15-30 seconds | Low |
| **Vote Credits** | Every 30-60 seconds | Low |
| **Vote Batches** | Every 60 seconds | Medium |
| **Validator Performance** | Every 30-60 seconds | Low-Medium |
| **Financial** | Every epoch (~2 days) | Low |
| **Network Activity** | Every 15-30 seconds | Low |

---

## 🚀 **UI IMPLEMENTATION PRIORITIES**

### **Phase 1: Core Dashboard (Must Have)**
- Node health status
- Vote credits performance
- Basic validator metrics
- Real-time alerts

### **Phase 2: Advanced Analysis (Should Have)**
- Vote batch detailed analysis
- Financial tracking
- Performance trends
- Historical data

### **Phase 3: Enterprise Features (Nice to Have)**
- Multi-validator comparison
- Advanced alerting
- Export/reporting
- Mobile responsiveness

---

**This catalog provides the complete foundation for building a comprehensive Solana validator monitoring UI with all available metrics organized for optimal user experience.**