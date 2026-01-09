# Vote Credits Monitoring Guide

This document explains how to monitor Solana validator vote credits and detect performance issues using the enhanced `solana-exporter` metrics.

## 📊 New Vote Credits Metrics

### Core Metrics

- **`solana_validator_epoch_credits`** - Vote credits earned by validator in current epoch
- **`solana_validator_expected_credits`** - Expected vote credits for validator in current epoch  
- **`solana_validator_credits_missed_pct`** - Percentage of vote credits missed by validator

### Existing Related Metrics

- **`solana_validator_last_vote`** - Last voted-on slot per validator
- **`solana_validator_delinquent`** - Whether validator is delinquent (0=active, 1=delinquent)
- **`solana_cluster_last_vote`** - Most recent voted-on slot of the cluster

## 🎯 Understanding Vote Credits

### What are Vote Credits?
- Validators earn 1 vote credit for each correct vote on a block
- Credits determine validator rewards and performance
- Missing votes = lost rewards and degraded network participation

### Performance Thresholds
- **Excellent**: >98% vote credit performance
- **Good**: 95-98% vote credit performance  
- **Warning**: 90-95% vote credit performance
- **Critical**: <90% vote credit performance

## 📈 Prometheus Queries for Monitoring

### 1. Current Vote Credit Performance

```promql
# Vote credit miss percentage for your validators
solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"}

# Vote credits earned vs expected
solana_validator_epoch_credits{nodekey="YOUR_NODEKEY"} / solana_validator_expected_credits{nodekey="YOUR_NODEKEY"} * 100
```

### 2. Vote Credit Trend Analysis

```promql
# Vote credit performance over time (rate of change)
rate(solana_validator_epoch_credits{nodekey="YOUR_NODEKEY"}[1h])

# Compare your performance to cluster average
solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"} - avg(solana_validator_credits_missed_pct)
```

### 3. Vote Lag Detection

```promql
# Slots behind in voting (current slot - last vote)
solana_node_slot_height - solana_validator_last_vote{nodekey="YOUR_NODEKEY"}

# Alert when vote lag exceeds threshold
(solana_node_slot_height - solana_validator_last_vote{nodekey="YOUR_NODEKEY"}) > 10
```

### 4. Performance Comparison

```promql
# Top 10 worst performing validators by vote credit miss rate
topk(10, solana_validator_credits_missed_pct)

# Your validator rank by performance
rank_desc(solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"})
```

## 🚨 Alerting Rules

### Prometheus Alerting Rules (`solana-vote-credits.yml`)

```yaml
groups:
  - name: solana-vote-credits
    rules:
    
    # Critical: High vote credit miss rate
    - alert: SolanaValidatorHighVoteMissRate
      expr: solana_validator_credits_missed_pct > 10
      for: 5m
      labels:
        severity: critical
        service: solana-validator
      annotations:
        summary: "Solana validator {{ $labels.nodekey }} has high vote miss rate"
        description: "Validator is missing {{ $value }}% of vote credits in current epoch"

    # Warning: Moderate vote credit miss rate  
    - alert: SolanaValidatorModerateLag
      expr: solana_validator_credits_missed_pct > 5 and solana_validator_credits_missed_pct <= 10
      for: 10m
      labels:
        severity: warning
        service: solana-validator
      annotations:
        summary: "Solana validator {{ $labels.nodekey }} has moderate vote lag"
        description: "Validator is missing {{ $value }}% of vote credits"

    # Critical: Vote lag too high
    - alert: SolanaValidatorVoteLag
      expr: (solana_node_slot_height - solana_validator_last_vote) > 20
      for: 2m
      labels:
        severity: critical
        service: solana-validator
      annotations:
        summary: "Solana validator {{ $labels.nodekey }} vote lag is too high"
        description: "Validator is {{ $value }} slots behind in voting"

    # Warning: Validator becoming delinquent
    - alert: SolanaValidatorDelinquent
      expr: solana_validator_delinquent == 1
      for: 1m
      labels:
        severity: warning
        service: solana-validator
      annotations:
        summary: "Solana validator {{ $labels.nodekey }} is delinquent"
        description: "Validator has been marked as delinquent by the cluster"
```

## 📊 Grafana Dashboard Panels

### Vote Credit Performance Panel

```json
{
  "title": "Vote Credit Performance",
  "type": "stat",
  "targets": [
    {
      "expr": "100 - solana_validator_credits_missed_pct{nodekey=\"$nodekey\"}",
      "legendFormat": "Vote Credit Performance %"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "percent",
      "min": 0,
      "max": 100,
      "thresholds": {
        "steps": [
          {"color": "red", "value": 0},
          {"color": "yellow", "value": 90},
          {"color": "green", "value": 98}
        ]
      }
    }
  }
}
```

### Vote Lag Time Series

```json
{
  "title": "Vote Lag Over Time",
  "type": "graph", 
  "targets": [
    {
      "expr": "solana_node_slot_height - solana_validator_last_vote{nodekey=\"$nodekey\"}",
      "legendFormat": "Vote Lag (slots)"
    }
  ],
  "yAxes": [
    {
      "label": "Slots",
      "min": 0
    }
  ],
  "alert": {
    "conditions": [
      {
        "query": {"queryType": "", "refId": "A"},
        "reducer": {"type": "last", "params": []},
        "evaluator": {"params": [10], "type": "gt"}
      }
    ],
    "executionErrorState": "alerting",
    "frequency": "10s",
    "handler": 1,
    "name": "Vote Lag Alert",
    "noDataState": "no_data"
  }
}
```

## 🔧 Troubleshooting Vote Credit Issues

### Common Causes of Vote Credit Misses

1. **Network Issues**
   - High latency to cluster
   - Packet loss
   - DNS resolution problems

2. **Hardware Problems**
   - CPU overload
   - Memory pressure  
   - Storage I/O bottleneck

3. **Configuration Issues**
   - Incorrect RPC endpoints
   - Firewall blocking ports
   - Clock synchronization

### Diagnostic Queries

```promql
# Check if node is healthy
solana_node_is_healthy{instance="YOUR_INSTANCE"}

# Check slots behind cluster
solana_node_num_slots_behind{instance="YOUR_INSTANCE"}

# Check if validator is active in consensus
solana_node_is_active{identity="YOUR_IDENTITY"}

# Transaction processing rate
rate(solana_node_transactions_total[5m])
```

### Recovery Actions

1. **Immediate Actions**
   - Check validator logs for errors
   - Verify network connectivity
   - Restart validator if necessary

2. **Performance Optimization**
   - Increase CPU/memory resources
   - Optimize network configuration
   - Update to latest Solana version

3. **Monitoring Setup**
   - Set up vote credit alerts
   - Monitor vote lag continuously
   - Track performance trends

## 📝 Example Configuration

### Solana Exporter Startup

```bash
# Run with vote credit monitoring enabled
./solana-exporter \
  -rpc-url http://localhost:8899 \
  -nodekey YOUR_VALIDATOR_IDENTITY \
  -votekey YOUR_VALIDATOR_VOTEKEY \
  -comprehensive-vote-account-tracking \
  -listen-address :8080
```

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 30s

scrape_configs:
  - job_name: 'solana-exporter'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 15s  # More frequent for vote credits
    
rule_files:
  - "solana-vote-credits.yml"
```

## 📚 Additional Resources

- [Solana Vote Account Documentation](https://docs.solana.com/validator/vote-accounts)
- [Solana RPC API Reference](https://docs.solana.com/api)
- [Prometheus Querying Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Alerting Guide](https://grafana.com/docs/grafana/latest/alerting/)

## 🎯 Best Practices

1. **Set Conservative Alerts**: Start with 5% miss rate, adjust based on network conditions
2. **Monitor Trends**: Look for degradation patterns, not just absolute values
3. **Compare to Peers**: Context matters - network-wide issues vs. validator-specific
4. **Automate Recovery**: Consider automatic restarts for persistent vote lag
5. **Historical Analysis**: Keep long-term data to identify recurring issues

---

*This monitoring setup helps maintain optimal validator performance and maximize staking rewards through proactive vote credit management.*