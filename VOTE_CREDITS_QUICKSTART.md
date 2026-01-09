# Vote Credits Monitoring - Quick Start Guide

This guide helps you quickly set up vote credits monitoring for your Solana validator.

## 🚀 Quick Setup (5 minutes)

### 1. Run Enhanced Solana Exporter

```bash
# Replace with your actual keys
./solana-exporter \
  -rpc-url http://localhost:8899 \
  -nodekey YOUR_VALIDATOR_IDENTITY_PUBKEY \
  -votekey YOUR_VALIDATOR_VOTE_PUBKEY \
  -comprehensive-vote-account-tracking \
  -listen-address :8080
```

### 2. Test New Metrics

```bash
# Check if vote credits metrics are available
curl -s http://localhost:8080/metrics | grep "solana_validator_credits"

# Should see:
# solana_validator_epoch_credits{votekey="...",nodekey="...",epoch="..."} 150
# solana_validator_expected_credits{votekey="...",nodekey="...",epoch="..."} 160  
# solana_validator_credits_missed_pct{votekey="...",nodekey="...",epoch="..."} 6.25
```

### 3. Basic Prometheus Query

```promql
# Your current vote performance percentage
100 - solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"}
```

## 🎯 Key Metrics at a Glance

| Metric | Good Value | Warning | Critical |
|--------|------------|---------|----------|
| Vote Performance % | >98% | 95-98% | <95% |
| Credits Missed % | <2% | 2-5% | >5% |
| Vote Lag (slots) | <5 | 5-20 | >20 |

## 📊 Essential Grafana Panels

### Performance Gauge
```json
{
  "title": "Vote Performance %",
  "type": "stat",
  "targets": [{
    "expr": "100 - solana_validator_credits_missed_pct{nodekey=\"$nodekey\"}"
  }],
  "fieldConfig": {
    "defaults": {
      "unit": "percent",
      "thresholds": {
        "steps": [
          {"color": "red", "value": 0},
          {"color": "yellow", "value": 95},
          {"color": "green", "value": 98}
        ]
      }
    }
  }
}
```

### Vote Lag Chart
```json
{
  "title": "Vote Lag",
  "type": "graph",
  "targets": [{
    "expr": "solana_node_slot_height - solana_validator_last_vote{nodekey=\"$nodekey\"}"
  }]
}
```

## 🚨 Basic Alert (Prometheus)

```yaml
# Add to your prometheus rules
- alert: ValidatorVotingIssues
  expr: solana_validator_credits_missed_pct > 5
  for: 5m
  annotations:
    summary: "Validator missing too many votes: {{ $value }}%"
```

## 🔍 Troubleshooting Commands

```bash
# Check validator health
curl -s http://localhost:8080/metrics | grep "solana_node_is_healthy"

# Check if behind cluster
curl -s http://localhost:8080/metrics | grep "solana_node_num_slots_behind"

# Check delinquent status
curl -s http://localhost:8080/metrics | grep "solana_validator_delinquent"
```

## 📱 Quick Health Check Script

```bash
#!/bin/bash
# save as check_validator.sh

EXPORTER_URL="http://localhost:8080/metrics"
NODEKEY="YOUR_VALIDATOR_IDENTITY"

echo "=== Validator Health Check ==="

# Vote performance
MISS_PCT=$(curl -s $EXPORTER_URL | grep "solana_validator_credits_missed_pct.*nodekey=\"$NODEKEY\"" | awk '{print $2}')
PERFORMANCE=$(echo "100 - $MISS_PCT" | bc)
echo "Vote Performance: ${PERFORMANCE}%"

# Vote lag
CURRENT_SLOT=$(curl -s $EXPORTER_URL | grep "solana_node_slot_height" | awk '{print $2}')
LAST_VOTE=$(curl -s $EXPORTER_URL | grep "solana_validator_last_vote.*nodekey=\"$NODEKEY\"" | awk '{print $2}')
LAG=$(echo "$CURRENT_SLOT - $LAST_VOTE" | bc)
echo "Vote Lag: $LAG slots"

# Health status
HEALTHY=$(curl -s $EXPORTER_URL | grep "solana_node_is_healthy" | awk '{print $2}')
echo "Node Healthy: $([ "$HEALTHY" = "1" ] && echo "YES" || echo "NO")"

# Summary
if (( $(echo "$PERFORMANCE > 98" | bc -l) )) && (( $(echo "$LAG < 10" | bc -l) )) && [ "$HEALTHY" = "1" ]; then
    echo "✅ Status: EXCELLENT"
elif (( $(echo "$PERFORMANCE > 95" | bc -l) )) && (( $(echo "$LAG < 20" | bc -l) )); then
    echo "⚠️  Status: GOOD"  
else
    echo "🚨 Status: NEEDS ATTENTION"
fi
```

## 🎛️ Environment Variables

```bash
# Optional: Set these in your environment
export SOLANA_VALIDATOR_NODEKEY="YOUR_IDENTITY_PUBKEY"
export SOLANA_VALIDATOR_VOTEKEY="YOUR_VOTE_PUBKEY"
export SOLANA_RPC_URL="http://localhost:8899"

# Run with env vars
./solana-exporter \
  -nodekey $SOLANA_VALIDATOR_NODEKEY \
  -votekey $SOLANA_VALIDATOR_VOTEKEY \
  -rpc-url $SOLANA_RPC_URL
```

## 📈 Next Steps

1. **Set up alerts**: Use the [detailed alerting guide](prometheus/solana-vote-credits-rules.yml)
2. **Create dashboard**: Import our [Grafana dashboard](prometheus/solana-dashboard.json)  
3. **Historical analysis**: Set up data retention for trend analysis
4. **Automation**: Consider auto-restart on persistent voting issues

## 🆘 Common Issues

### Vote lag is high but node is healthy
- Check network connectivity to cluster
- Verify RPC endpoint performance
- Monitor CPU/memory usage

### Credits missed but vote lag is low  
- Check for intermittent network issues
- Verify stake account is active
- Review validator logs for errors

### Sudden performance drop
- Check cluster-wide issues first
- Verify validator hasn't been jailed
- Monitor stake changes

---

**Need help?** Check the [full monitoring guide](vote-credits-monitoring-example.md) or review [Solana validator documentation](https://docs.solana.com/running-validator).