# Solana Exporter Metrics - Quick Reference

## 🚨 Critical Alerts (Must Monitor)

| Metric | Good | Warning | Critical | Query |
|--------|------|---------|----------|-------|
| `solana_validator_credits_missed_pct` | <2% | 2-5% | >5% | `solana_validator_credits_missed_pct{nodekey="YOUR_KEY"}` |
| `solana_vote_batch_performance_pct` | >95% | 90-95% | <90% | `solana_vote_batch_performance_pct{nodekey="YOUR_KEY"}` |
| `solana_node_is_healthy` | 1 | - | 0 | `solana_node_is_healthy` |
| `solana_validator_delinquent` | 0 | - | 1 | `solana_validator_delinquent{nodekey="YOUR_KEY"}` |
| `solana_node_num_slots_behind` | <10 | 10-50 | >50 | `solana_node_num_slots_behind` |
| `solana_vote_batch_avg_latency_seconds` | <3.5s | 3.5-5s | >5s | `solana_vote_batch_avg_latency_seconds{nodekey="YOUR_KEY"}` |

## 📊 Performance Dashboard

| Metric | Description | Key Query |
|--------|-------------|-----------|
| **Vote Performance** | `100 - solana_validator_credits_missed_pct` | Vote credit success rate |
| **Batch Performance** | `solana_vote_batch_performance_pct` | Granular batch-level voting efficiency |
| **Vote Lag** | `solana_node_slot_height - solana_validator_last_vote` | Slots behind in voting |
| **Batch Latency** | `solana_vote_batch_avg_latency_seconds` | Average voting response time per batch |
| **Skip Rate** | `rate(leader_slots{status="skipped"}[1h]) / rate(leader_slots[1h])` | Block production failures |
| **Active Stake** | `solana_validator_active_stake` | Validator stake amount |

## 💰 Revenue Tracking

| Metric | Type | Purpose |
|--------|------|---------|
| `solana_validator_inflation_rewards_total` | Counter | Staking rewards earned |
| `solana_validator_fee_rewards_total` | Counter | Transaction fee earnings |
| `solana_validator_commission` | Gauge | Current commission rate |
| `solana_account_balance` | Gauge | Wallet balances |

## 🔍 Vote Batch Analysis

| Metric | Type | Purpose |
|--------|------|---------|
| `solana_vote_batch_missed_tvcs` | Gauge | TVCs missed per batch |
| `solana_vote_batch_missed_slots` | Gauge | Slots missed per batch |
| `solana_vote_batch_avg_latency_seconds` | Gauge | Average batch voting latency |
| `solana_vote_batch_performance_pct` | Gauge | Batch-level vote performance |

## 🔍 Troubleshooting Commands

```bash
# Quick health check
curl -s localhost:8080/metrics | grep "solana_node_is_healthy"

# Vote performance  
curl -s localhost:8080/metrics | grep "credits_missed_pct"

# Vote batch performance
curl -s localhost:8080/metrics | grep "vote_batch_performance_pct"

# Check if behind
curl -s localhost:8080/metrics | grep "slots_behind"

# Batch latency check
curl -s localhost:8080/metrics | grep "vote_batch_avg_latency"

# Version check
curl -s localhost:8080/metrics | grep "version_outdated"
```

## ⚡ Essential Prometheus Queries

### Validator Health
```promql
# Vote performance percentage
100 - solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"}

# Is validator healthy?
solana_node_is_healthy == 1

# Vote lag in slots
solana_node_slot_height - solana_validator_last_vote{nodekey="YOUR_NODEKEY"}

# Vote batch performance
solana_vote_batch_performance_pct{nodekey="YOUR_NODEKEY"}

# Identify worst performing batches
topk(5, solana_vote_batch_missed_tvcs{nodekey="YOUR_NODEKEY"})
```

### Performance Analysis
```promql
# Skip rate last hour
rate(solana_validator_leader_slots_total{status="skipped",nodekey="YOUR_NODEKEY"}[1h]) / 
rate(solana_validator_leader_slots_total{nodekey="YOUR_NODEKEY"}[1h]) * 100

# Rewards last 24h
increase(solana_validator_inflation_rewards_total{votekey="YOUR_VOTEKEY"}[24h])

# Compare to cluster average
solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"} - avg(solana_validator_credits_missed_pct)

# Vote batch latency trends
avg_over_time(solana_vote_batch_avg_latency_seconds{nodekey="YOUR_NODEKEY"}[1h])

# Batches with high missed TVCs
solana_vote_batch_missed_tvcs{nodekey="YOUR_NODEKEY"} > 2000
```

## 🎯 Alert Thresholds (Recommended)

```yaml
# Critical: High vote miss rate
solana_validator_credits_missed_pct > 10

# Warning: Moderate vote issues  
solana_validator_credits_missed_pct > 5

# Critical: Vote lag too high
(solana_node_slot_height - solana_validator_last_vote) > 20

# Warning: Node unhealthy
solana_node_is_healthy == 0

# Critical: Validator delinquent
solana_validator_delinquent == 1

# Warning: Batch performance issues
solana_vote_batch_performance_pct < 90

# Critical: High batch latency
solana_vote_batch_avg_latency_seconds > 10
```

## 📋 Configuration Flags Impact

| Flag | Metrics Added | Resource Impact | Use When |
|------|--------------|-----------------|----------|
| `-nodekey` | Per-validator metrics | Low | Always (your validators) |
| `-votekey` | Vote account metrics | Low | Monitoring unstaked validators |
| `-comprehensive-vote-account-tracking` | All validators | Medium | Cluster-wide analysis |
| `-comprehensive-slot-tracking` | Thousands per epoch | High | Detailed leader analysis |
| `-monitor-block-sizes` | Block size metrics | Medium | MEV/block analysis |
| *(Vote batch analysis)* | Batch metrics | Low | Automatic with nodekey/votekey |

## 🚀 One-Liner Health Check

```bash
#!/bin/bash
# Validator health summary
echo "Health: $(curl -s localhost:8080/metrics | grep 'solana_node_is_healthy' | awk '{print $2 == 1 ? "✅ GOOD" : "❌ BAD"}')"
echo "Vote Performance: $(curl -s localhost:8080/metrics | grep 'credits_missed_pct.*nodekey="'$1'"' | awk '{print 100-$2 "%"}')"
echo "Vote Lag: $(curl -s localhost:8080/metrics | grep 'solana_node_slot_height' | awk '{slot=$2} END{getline vote < "curl -s localhost:8080/metrics | grep last_vote.*nodekey=\"'$1'\""; print slot-vote " slots"}')"
echo "Latest Batch Performance: $(curl -s localhost:8080/metrics | grep 'vote_batch_performance_pct.*nodekey="'$1'"' | tail -1 | awk '{print $2 "%"}')"
```

## 📚 Key Files Reference

- **Comprehensive Guide**: `vote-credits-monitoring-example.md`
- **Full Metrics List**: `METRICS_SUMMARY.md` 
- **Quick Start**: `VOTE_CREDITS_QUICKSTART.md`
- **Alert Rules**: `prometheus/solana-vote-credits-rules.yml`

## 🔥 Emergency Response

**If vote credits drop below 90%:**
1. Check `solana_node_is_healthy`
2. Check `solana_node_num_slots_behind` 
3. Review validator logs
4. Consider validator restart
5. Monitor recovery with vote performance queries

**If validator goes delinquent:**
1. Immediate alert via `solana_validator_delinquent == 1`
2. Check stake account status
3. Verify network connectivity
4. Restart validator if needed
5. Monitor return to active status