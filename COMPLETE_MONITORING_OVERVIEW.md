# Solana Exporter - Complete Monitoring Solution Overview

## 🎯 Executive Summary

Solana Exporter is a comprehensive monitoring solution for Solana validators and networks, providing 26+ Prometheus metrics covering validator performance, vote credits tracking, network health, and revenue monitoring. The enhanced version includes advanced **Vote Credits (TVC) monitoring** for detecting and preventing validator performance issues.

## 🚀 What We Monitor

### 🏆 Core Validator Performance
- **Vote Credit Performance** - Track voting efficiency and missed opportunities
- **Stake Management** - Monitor active stake and delegation changes  
- **Block Production** - Skip rate, block sizes, and leader slot performance
- **Revenue Tracking** - Inflation rewards and transaction fee earnings
- **Delinquency Detection** - Real-time validator status monitoring

### 🌐 Network & Node Health  
- **Consensus Participation** - Vote lag, root slot tracking
- **Sync Status** - Slots behind cluster, health checks
- **Version Management** - Outdated software detection
- **Ledger Health** - Data availability and retention
- **Transaction Processing** - Network activity monitoring

### 💰 Financial Monitoring
- **Wallet Balances** - SOL balance tracking for any addresses
- **Reward Analysis** - Epoch-by-epoch earnings breakdown
- **Commission Tracking** - Rate changes and optimization
- **Treasury Management** - Operational wallet monitoring

## 🔥 Key Features & Enhancements

### ⭐ **NEW: Advanced Vote Credits Monitoring**
```
🎯 EPOCH-LEVEL METRICS:
• solana_validator_credits_missed_pct - % of votes missed
• solana_validator_epoch_credits - Actual credits earned  
• solana_validator_expected_credits - Expected performance baseline

🔍 BATCH-LEVEL GRANULAR METRICS:
• solana_vote_batch_missed_tvcs - TVCs missed per batch
• solana_vote_batch_performance_pct - Performance per batch
• solana_vote_batch_avg_latency_seconds - Voting latency per batch
• solana_vote_batch_missed_slots - Slots missed per batch

🚨 CRITICAL ALERTS:
• >5% miss rate = Performance issues
• >10% miss rate = Revenue impact
• >20% miss rate = Delinquency risk
• Batch latency >5s = Network issues
```

### 🎛️ Flexible Configuration
- **Light Mode** - Minimal metrics for basic monitoring
- **Comprehensive Tracking** - Full network visibility (thousands of metrics)
- **Custom Validators** - Monitor any validator by nodekey/votekey
- **Multi-Address** - Track multiple wallets and accounts

### 📊 Production-Ready
- **Prometheus Integration** - Native metrics export
- **Grafana Dashboards** - Pre-built visualization templates  
- **Alert Rules** - Ready-to-use Prometheus alerting
- **Docker Support** - Containerized deployment
- **High Availability** - Fault-tolerant design

## 📈 Monitoring Categories

| Category | Metrics Count | Key Use Cases |
|----------|---------------|---------------|
| **Vote Credits & Batches** | 9 | TVC tracking, granular batch analysis, latency monitoring |
| **Validator Performance** | 8 | Skip rate, stake, rewards, delinquency |
| **Node Health** | 8 | Sync status, version, connectivity |
| **Network Activity** | 7 | Slots, epochs, transactions, blocks |
| **Financial** | 4 | Balances, rewards, commission tracking |

## 🎯 Critical Monitoring Workflows

### 1. **Vote Credits Performance (TVC) 🗳️**
```promql
# Epoch-level vote performance
100 - solana_validator_credits_missed_pct{nodekey="YOUR_KEY"}

# Batch-level granular performance  
solana_vote_batch_performance_pct{nodekey="YOUR_KEY"}

# Vote batch latency monitoring
solana_vote_batch_avg_latency_seconds{nodekey="YOUR_KEY"}

# Identify problematic batches
topk(5, solana_vote_batch_missed_tvcs{nodekey="YOUR_KEY"})

# Performance threshold alerts:
# >98% = Excellent  |  95-98% = Good  |  90-95% = Warning  |  <90% = Critical
```

### 2. **Validator Health Check ✅**
```promql
# Core health indicators
solana_node_is_healthy == 1
solana_validator_delinquent{nodekey="YOUR_KEY"} == 0  
solana_node_num_slots_behind < 10
```

### 3. **Revenue Optimization 💰**
```promql
# Daily earnings tracking
increase(solana_validator_inflation_rewards_total[24h]) + 
increase(solana_validator_fee_rewards_total[24h])
```

### 4. **Performance Benchmarking 📊**
```promql
# Skip rate calculation
rate(solana_validator_leader_slots_total{status="skipped"}[1h]) / 
rate(solana_validator_leader_slots_total[1h]) * 100
```

## 🚨 Alert Strategy

### **Tier 1: Critical (Immediate Response)**
- Vote credits missed >10% → Revenue impact
- Validator delinquent → No rewards
- Node unhealthy → Service degradation  
- Vote lag >50 slots → Consensus issues

### **Tier 2: Warning (Monitor Closely)**  
- Vote credits missed 5-10% → Performance degradation
- Node 10-50 slots behind → Sync issues
- Skip rate increasing → Block production problems
- Version outdated → Security/performance risks

### **Tier 3: Info (Trending Analysis)**
- Vote performance below cluster average
- Stake changes detected
- Epoch transitions
- Weekly performance summaries

## 📋 Deployment Options

### **Quick Start (5 minutes)**
```bash
./solana-exporter \
  -nodekey YOUR_VALIDATOR_IDENTITY \
  -votekey YOUR_VALIDATOR_VOTE_ACCOUNT \
  -rpc-url http://localhost:8899 \
  -listen-address :8080
```

### **Production Configuration**
```bash
./solana-exporter \
  -nodekey VALIDATOR_1 -nodekey VALIDATOR_2 \
  -votekey VOTEKEY_1 -votekey VOTEKEY_2 \
  -balance-address TREASURY_WALLET \
  -comprehensive-vote-account-tracking \
  -monitor-block-sizes \
  -active-identity MY_VALIDATOR_IDENTITY \
  -rpc-url http://localhost:8899 \
  -reference-rpc-url https://api.mainnet-beta.solana.com \
  -listen-address :8080
```

### **Enterprise Docker Setup**
```dockerfile
FROM ghcr.io/asymmetric-research/solana-exporter:latest
COPY config.yml /etc/solana-exporter/
EXPOSE 8080
CMD ["solana-exporter", "-config", "/etc/solana-exporter/config.yml"]
```

## 🎛️ Configuration Matrix

| Feature | Flag | Resource Impact | Best For |
|---------|------|-----------------|----------|
| **Basic Monitoring** | `-nodekey`, `-votekey` | ⚡ Low | Single validator operators |
| **Vote Credits Tracking** | *Automatic with nodekey* | ⚡ Low | All deployments |
| **Multi-Validator** | `-comprehensive-vote-account-tracking` | 🔶 Medium | Staking services |
| **Block Analysis** | `-monitor-block-sizes` | 🔶 Medium | MEV/performance optimization |
| **Full Network** | `-comprehensive-slot-tracking` | 🔴 High | Research/analytics |

## 📊 Dashboard Templates

### **Executive Dashboard**
- Vote Credit Performance (primary KPI)
- Validator Health Status
- Daily/Monthly Revenue
- Network Position (rank, stake)

### **Operations Dashboard**  
- Real-time vote lag
- Skip rate trends
- Node health indicators
- Alert status overview

### **Financial Dashboard**
- Reward earnings breakdown
- Commission rate tracking  
- Wallet balance monitoring
- ROI analysis

## 🔧 Troubleshooting Guide

### **Vote Credits Issues**
1. **High Miss Rate (>5%)**
   - Check network connectivity
   - Monitor CPU/memory usage
   - Verify RPC endpoint health
   - Review validator logs

2. **Vote Lag (>20 slots)**
   - Check `solana_node_num_slots_behind`
   - Verify network latency
   - Monitor disk I/O
   - Consider hardware upgrade

### **Common Performance Fixes**
- **Network issues**: Optimize RPC endpoints, check firewall
- **Resource constraints**: Scale CPU/memory, monitor disk space  
- **Configuration**: Verify keys, check stake account status
- **Version**: Update to latest Solana release

## 🎯 Success Metrics

### **Excellent Performance**
- Vote credits missed: <2%
- Skip rate: <5%
- Vote lag: <5 slots
- Uptime: >99.9%

### **Revenue Optimization**
- Consistent epoch rewards
- Optimal commission rates
- Minimized missed opportunities
- Competitive performance ranking

## 📚 Complete Documentation

| Document | Purpose | Audience |
|----------|---------|----------|
| `METRICS_SUMMARY.md` | Complete metrics reference | Technical teams |
| `VOTE_CREDITS_QUICKSTART.md` | 5-minute setup guide | Operators |
| `vote-credits-monitoring-example.md` | Detailed implementation | DevOps engineers |
| `METRICS_CHEAT_SHEET.md` | Quick reference | All users |
| `prometheus/solana-vote-credits-rules.yml` | Production alerts | SRE teams |

## 🏆 Why Solana Exporter?

✅ **Complete Coverage** - 26+ metrics covering all aspects  
✅ **Vote Credits Focus** - Advanced TVC monitoring (industry-first)  
✅ **Production Ready** - Battle-tested, high-availability design  
✅ **Easy Integration** - Prometheus/Grafana native support  
✅ **Flexible Deployment** - From single validator to enterprise  
✅ **Active Development** - Continuously updated with network changes  
✅ **Open Source** - Transparent, community-driven  

## 🎯 Next Steps

1. **Quick Start**: Deploy basic monitoring in 5 minutes
2. **Configure Alerts**: Set up vote credits performance alerts  
3. **Build Dashboard**: Create operational visibility
4. **Optimize Performance**: Use metrics to improve validator efficiency
5. **Scale Monitoring**: Add comprehensive tracking as needed

---

**Solana Exporter provides the most comprehensive validator monitoring solution available, with industry-leading Vote Credits tracking AND granular Vote Batch analysis that helps operators maximize performance and rewards while minimizing risks.**

*Ready to optimize your Solana validator performance? Start with the Quick Start guide and achieve >98% vote credit performance with detailed batch-level insights within hours.*