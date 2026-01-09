# Vote Batch Detailed Monitoring - Advanced TVC Analysis

This document describes the advanced vote batch monitoring capabilities that provide granular insight into validator voting patterns, similar to the detailed analysis shown in professional Solana monitoring tools.

## 🎯 Overview

Vote Batch Monitoring analyzes validator voting patterns in detailed batches, identifying specific time periods, slot ranges, latency, and missed TVCs. This provides the granular data needed to optimize validator performance and troubleshoot voting issues.

## 📊 Vote Batch Metrics

### Core Vote Batch Metrics

#### **`solana_vote_batch_missed_tvcs`** *(Gauge)*
- **Description**: Number of missed TVCs per vote batch
- **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
- **Example**: `solana_vote_batch_missed_tvcs{nodekey="7Np41...", votekey="84hoQ...", batch="1", slot_range="392257999-392259999"} 2335`
- **Use Case**: Identify specific batches with high TVC misses

#### **`solana_vote_batch_missed_slots`** *(Gauge)*
- **Description**: Number of missed slots per vote batch
- **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
- **Example**: `solana_vote_batch_missed_slots{nodekey="7Np41...", batch="2", slot_range="392259999-392261999"} 0`
- **Use Case**: Track slot-level voting gaps

#### **`solana_vote_batch_avg_latency_seconds`** *(Gauge)*
- **Description**: Average voting latency per batch in seconds
- **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
- **Example**: `solana_vote_batch_avg_latency_seconds{batch="3", slot_range="392261999-392263999"} 3.18`
- **Use Case**: Monitor voting response times and network performance

#### **`solana_vote_batch_performance_pct`** *(Gauge)*
- **Description**: Vote performance percentage per batch (voted slots / total slots * 100)
- **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
- **Example**: `solana_vote_batch_performance_pct{batch="4", slot_range="392263999-392265999"} 95.5`
- **Use Case**: **PRIMARY BATCH PERFORMANCE METRIC** - track batch-level efficiency

#### **`solana_vote_batch_slot_range`** *(Gauge)*
- **Description**: Total number of slots covered by vote batch
- **Labels**: `nodekey`, `votekey`, `batch`, `slot_range`
- **Example**: `solana_vote_batch_slot_range{batch="5", slot_range="392265999-392267999"} 2000`
- **Use Case**: Understand batch sizing and coverage

#### **`solana_vote_batch_count_total`** *(Gauge)*
- **Description**: Total number of vote batches analyzed
- **Labels**: `nodekey`, `votekey`
- **Example**: `solana_vote_batch_count_total{nodekey="7Np41...", votekey="84hoQ..."} 13`
- **Use Case**: Track voting pattern consistency

## 🔍 Understanding Vote Batch Data

### What is a Vote Batch?

Vote batches are groups of consecutive votes that validators submit together. Instead of voting on every single slot, validators group slots into batches for efficiency.

**Batch Characteristics:**
- **Batch ID**: Sequential number (1, 2, 3, ...)
- **Slot Range**: Start and end slots (e.g., "392257999-392259999")
- **Time Period**: When the batch was processed
- **Performance**: Percentage of slots successfully voted on
- **Latency**: Average time delay in voting
- **Missed TVCs**: Vote credits lost in this batch

### Example Vote Batch Analysis

```
Batch | Start Time           | Slot Range     | Avg Latency | Missed TVCs | Missed Slots
------|---------------------|----------------|-------------|-------------|-------------
1     | 09.01.2026 02:26:56 | 392257999     | 3.18        | 2335        | 0
2     | 09.01.2026 02:40:16 | 392259999     | 3.21        | 2401        | 0
3     | 09.01.2026 02:53:36 | 392261999     | 3.20        | 2374        | 0
4     | 09.01.2026 03:06:56 | 392263999     | 3.18        | 2316        | 0
5     | 09.01.2026 03:20:16 | 392265999     | 3.24        | 2432        | 0
```

## 📈 Essential Prometheus Queries

### Batch Performance Analysis

```promql
# Performance of recent vote batches
solana_vote_batch_performance_pct{nodekey="YOUR_NODEKEY"}

# Identify worst performing batches
topk(5, solana_vote_batch_missed_tvcs{nodekey="YOUR_NODEKEY"})

# Average latency across all batches
avg(solana_vote_batch_avg_latency_seconds{nodekey="YOUR_NODEKEY"})

# Batches with zero missed slots (perfect performance)
count(solana_vote_batch_missed_slots{nodekey="YOUR_NODEKEY"} == 0)
```

### Trend Analysis

```promql
# Latency trend over time
avg_over_time(solana_vote_batch_avg_latency_seconds{nodekey="YOUR_NODEKEY"}[1h])

# Performance degradation detection
delta(solana_vote_batch_performance_pct{nodekey="YOUR_NODEKEY"}[30m])

# Missed TVC rate per batch
rate(solana_vote_batch_missed_tvcs{nodekey="YOUR_NODEKEY"}[1h])
```

### Comparative Analysis

```promql
# Compare batch performance to overall performance
solana_vote_batch_performance_pct{nodekey="YOUR_NODEKEY"} - 
(100 - solana_validator_credits_missed_pct{nodekey="YOUR_NODEKEY"})

# Identify batches with high latency
solana_vote_batch_avg_latency_seconds{nodekey="YOUR_NODEKEY"} > 5.0
```

## 🚨 Advanced Alerting Rules

### Batch-Level Alerts

```yaml
groups:
  - name: solana-vote-batch-alerts
    rules:

    # Critical: Batch with excessive missed TVCs
    - alert: SolanaVoteBatchHighMissedTVCs
      expr: solana_vote_batch_missed_tvcs > 3000
      for: 1m
      labels:
        severity: critical
        service: solana-validator
        alert_type: vote_batch
      annotations:
        summary: "Vote batch {{ $labels.batch }} has high missed TVCs"
        description: |
          Batch {{ $labels.batch }} ({{ $labels.slot_range }}) missed {{ $value }} TVCs.
          This indicates significant voting issues during this period.

    # Warning: Batch performance below threshold
    - alert: SolanaVoteBatchLowPerformance
      expr: solana_vote_batch_performance_pct < 90
      for: 2m
      labels:
        severity: warning
        service: solana-validator
        alert_type: vote_batch
      annotations:
        summary: "Vote batch {{ $labels.batch }} has low performance"
        description: |
          Batch {{ $labels.batch }} performance: {{ printf "%.2f" $value }}%
          Slot range: {{ $labels.slot_range }}
          Consider investigating network or hardware issues.

    # Critical: High voting latency
    - alert: SolanaVoteBatchHighLatency
      expr: solana_vote_batch_avg_latency_seconds > 10
      for: 1m
      labels:
        severity: critical
        service: solana-validator
        alert_type: vote_batch
      annotations:
        summary: "Vote batch {{ $labels.batch }} has high latency"
        description: |
          Average latency: {{ printf "%.2f" $value }}s
          Slot range: {{ $labels.slot_range }}
          High latency may indicate network or processing issues.

    # Warning: Missed slots in batch
    - alert: SolanaVoteBatchMissedSlots
      expr: solana_vote_batch_missed_slots > 0
      for: 5m
      labels:
        severity: warning
        service: solana-validator
        alert_type: vote_batch
      annotations:
        summary: "Vote batch {{ $labels.batch }} has missed slots"
        description: |
          Missed {{ $value }} slots in batch {{ $labels.batch }}
          Slot range: {{ $labels.slot_range }}
          Monitor for pattern of missed slots.
```

## 📊 Grafana Dashboard Panels

### Vote Batch Performance Table

```json
{
  "title": "Vote Batch Details",
  "type": "table",
  "targets": [
    {
      "expr": "solana_vote_batch_performance_pct{nodekey=\"$nodekey\"}",
      "format": "table",
      "legendFormat": "Performance %"
    },
    {
      "expr": "solana_vote_batch_missed_tvcs{nodekey=\"$nodekey\"}",
      "format": "table", 
      "legendFormat": "Missed TVCs"
    },
    {
      "expr": "solana_vote_batch_avg_latency_seconds{nodekey=\"$nodekey\"}",
      "format": "table",
      "legendFormat": "Avg Latency"
    }
  ],
  "transformations": [
    {
      "id": "merge",
      "options": {}
    },
    {
      "id": "organize",
      "options": {
        "excludeByName": {},
        "indexByName": {},
        "renameByName": {
          "batch": "Batch",
          "slot_range": "Slot Range", 
          "Value #A": "Performance %",
          "Value #B": "Missed TVCs",
          "Value #C": "Avg Latency (s)"
        }
      }
    }
  ]
}
```

### Batch Latency Time Series

```json
{
  "title": "Vote Batch Latency Over Time",
  "type": "graph",
  "targets": [
    {
      "expr": "solana_vote_batch_avg_latency_seconds{nodekey=\"$nodekey\"}",
      "legendFormat": "Batch {{batch}} ({{slot_range}})"
    }
  ],
  "yAxes": [
    {
      "label": "Latency (seconds)",
      "min": 0,
      "max": 10
    }
  ],
  "thresholds": [
    {
      "value": 5.0,
      "colorMode": "critical",
      "op": "gt"
    },
    {
      "value": 3.5,
      "colorMode": "warning",
      "op": "gt"
    }
  ]
}
```

### Batch Performance Heatmap

```json
{
  "title": "Vote Batch Performance Heatmap",
  "type": "heatmap",
  "targets": [
    {
      "expr": "solana_vote_batch_performance_pct{nodekey=\"$nodekey\"}",
      "format": "time_series",
      "legendFormat": "{{batch}}"
    }
  ],
  "heatmap": {
    "xBucketSize": "1h",
    "yBucketSize": 5,
    "colorScale": "linear",
    "colorScheme": "RdYlGn",
    "minValue": 0,
    "maxValue": 100
  }
}
```

## 🔧 Configuration and Tuning

### Vote Batch Analysis Parameters

```go
const (
    MaxVoteBatchGap   = 50  // Maximum slots between votes in same batch
    MinBatchSize      = 100 // Minimum slots to consider as a batch  
    MaxLatencySeconds = 30  // Maximum expected voting latency
)
```

### Enabling Vote Batch Monitoring

```bash
# Vote batch analysis is automatically enabled when monitoring validators
./solana-exporter \
  -nodekey YOUR_VALIDATOR_IDENTITY \
  -votekey YOUR_VALIDATOR_VOTE_ACCOUNT \
  -comprehensive-vote-account-tracking \
  -rpc-url http://localhost:8899
```

### Performance Considerations

- **Resource Usage**: Vote batch analysis requires additional RPC calls
- **Memory**: Each batch stores vote history data
- **Update Frequency**: Batches are analyzed on each collection cycle
- **Data Retention**: Consider batch data cleanup for long-running deployments

## 🎯 Troubleshooting with Vote Batch Data

### Identifying Issues

1. **High Missed TVCs in Specific Batches**
   - Check network connectivity during batch time period
   - Correlate with node health metrics
   - Review validator logs for errors

2. **Increasing Latency Trends**
   - Monitor network latency to cluster
   - Check CPU/memory usage during high latency periods  
   - Verify RPC endpoint performance

3. **Inconsistent Batch Performance**
   - Look for patterns in missed slots
   - Check for intermittent hardware issues
   - Monitor stake changes that might affect voting

### Recovery Strategies

1. **Short-term Fixes**
   - Restart validator during low-performance batches
   - Switch to backup RPC endpoints
   - Increase resource allocation

2. **Long-term Optimization**
   - Optimize network configuration
   - Upgrade hardware based on latency patterns
   - Implement automated recovery based on batch performance

## 📋 Best Practices

### Monitoring Strategy

1. **Set Progressive Alerts**
   - Info: Batch performance <95%
   - Warning: Batch performance <90%  
   - Critical: Batch performance <80%

2. **Track Trends**
   - Monitor batch-to-batch consistency
   - Identify time-of-day patterns
   - Correlate with network conditions

3. **Optimize Based on Data**
   - Use latency data to tune network
   - Adjust batch size based on performance
   - Implement predictive maintenance

### Integration with Existing Monitoring

Vote batch metrics complement existing TVC monitoring:

- **Epoch-level**: `solana_validator_credits_missed_pct`
- **Real-time**: `solana_validator_last_vote` lag
- **Batch-level**: `solana_vote_batch_performance_pct`

This provides complete coverage from high-level trends to granular batch analysis.

---

**Vote Batch Detailed Monitoring provides the most granular view of validator voting performance, enabling precise identification and resolution of TVC-related issues.**