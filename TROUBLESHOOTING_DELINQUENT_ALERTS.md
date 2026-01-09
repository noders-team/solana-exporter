# Troubleshooting: Delinquent Validator Alerts Issue

## 🚨 Problem Description

After upgrading Solana Exporter with Web UI, Alert Manager started sending alerts for ALL delinquent validators in the Solana network, not just your own validators.

## 🔍 Root Cause Analysis

The issue was introduced by adding the `-comprehensive-vote-account-tracking` flag to the systemd service configuration. This flag changes the monitoring scope from "your validators only" to "all validators in the network".

### Before (Original Configuration)
```bash
ExecStart=/home/solana/solana-exporter/solana-exporter \
  -nodekey "$NODEKEY" \
  -active-identity "$ACTIVE_IDENTITY" \
  -balance-address "$BALANCE_ADDRESS" \
  -monitor-block-sizes \
  -rpc-url "$RPC_URL"
```
**Result**: Monitored only YOUR validators specified in nodekey

### After (Problematic Configuration)  
```bash
ExecStart=/home/solana/solana-exporter/solana-exporter \
  -nodekey "$NODEKEY" \
  -votekey "$VOTEKEY" \
  -active-identity "$ACTIVE_IDENTITY" \
  -balance-address "$BALANCE_ADDRESS" \
  -monitor-block-sizes \
  -comprehensive-vote-account-tracking \  # ← PROBLEM FLAG
  -rpc-url "$RPC_URL"
```
**Result**: Monitored ALL validators in the network (thousands of them)

## 📊 Technical Explanation

The `-comprehensive-vote-account-tracking` flag modifies the condition in the collector code:

```go
// Before: Only your validators
if slices.Contains(c.config.Nodekeys, account.NodePubkey) {
    ch <- c.ValidatorDelinquent.MustNewConstMetric(...)
}

// After: Your validators + ALL validators if comprehensive tracking enabled
if slices.Contains(c.config.Nodekeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
    ch <- c.ValidatorDelinquent.MustNewConstMetric(...)
}
```

This means that when comprehensive tracking is enabled:
- Metrics are generated for ALL ~1,900 validators in the network
- Including ALL delinquent validators (typically 50-200 at any time)
- Your alert rules trigger for every single delinquent validator

## 🔧 Solution Options

### Option 1: Remove Comprehensive Tracking (Recommended)

**Quick Fix:**
```bash
sudo systemctl stop solana-exporter

# Remove the problematic flag
sudo sed -i 's/-comprehensive-vote-account-tracking/# -comprehensive-vote-account-tracking/' /etc/systemd/system/solana-exporter.service

sudo systemctl daemon-reload
sudo systemctl start solana-exporter
```

**Complete Fix:**
```bash
sudo tee /etc/systemd/system/solana-exporter.service > /dev/null << 'EOF'
[Unit]
Description=Solana Exporter with Web UI (Fixed)
Documentation=https://github.com/noders-team/solana-exporter
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=solana
Group=solana
WorkingDirectory=/home/solana/solana-exporter
EnvironmentFile=/home/solana/.env

# Fixed configuration WITHOUT comprehensive tracking
ExecStart=/home/solana/solana-exporter/solana-exporter \
  -nodekey "${NODEKEY}" \
  -votekey "${VOTEKEY}" \
  -active-identity "${ACTIVE_IDENTITY}" \
  -balance-address "${BALANCE_ADDRESS}" \
  -monitor-block-sizes \
  -rpc-url "${RPC_URL}" \
  -listen-address "${LISTEN_ADDRESS}" \
  -reference-rpc-url "${REFERENCE_RPC_URL}"

Restart=always
RestartSec=5s
TimeoutStartSec=60s
TimeoutStopSec=30s

StandardOutput=journal
StandardError=journal
SyslogIdentifier=solana-exporter

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl restart solana-exporter
```

### Option 2: Update Alert Rules (Alternative)

If you want to keep comprehensive tracking but fix alerts, update your Prometheus alert rules:

**Before:**
```yaml
- alert: ValidatorDelinquent
  expr: solana_validator_delinquent == 1
  labels:
    severity: critical
  annotations:
    summary: "Validator is delinquent"
```

**After:**
```yaml
- alert: ValidatorDelinquent
  expr: solana_validator_delinquent{nodekey="YOUR_VALIDATOR_NODEKEY_HERE"} == 1
  labels:
    severity: critical
  annotations:
    summary: "Your validator {{ $labels.nodekey }} is delinquent"
```

### Option 3: Prometheus Metric Filtering

Add metric filtering in prometheus.yml:

```yaml
scrape_configs:
  - job_name: 'solana-exporter'
    static_configs:
      - targets: ['localhost:8080']
    metric_relabel_configs:
      # Only keep metrics for your validators
      - source_labels: [__name__, nodekey]
        regex: 'solana_validator_delinquent;(?!YOUR_VALIDATOR_NODEKEY_HERE).*'
        action: drop
```

## ✅ Verification Steps

After applying the fix:

1. **Check metric count:**
```bash
# Before fix: hundreds of delinquent metrics
curl -s http://localhost:8080/metrics | grep "solana_validator_delinquent" | wc -l

# After fix: only your validators (1-2 metrics typically)
curl -s http://localhost:8080/metrics | grep "solana_validator_delinquent" | wc -l
```

2. **Check specific metrics:**
```bash
# Should only show your validator(s)
curl -s http://localhost:8080/metrics | grep "solana_validator_delinquent"
```

3. **Monitor alerts:**
```bash
# Check that alerts stopped firing
# (Check your AlertManager web interface)
```

## 📊 Impact Comparison

### With Comprehensive Tracking
- ✅ Monitor entire Solana network
- ✅ Research and analytics capabilities
- ❌ Thousands of unnecessary metrics
- ❌ False alerts for other validators
- ❌ Higher resource usage
- ❌ Alert noise

### Without Comprehensive Tracking (Recommended)
- ✅ Monitor only your validators
- ✅ All TVC features for your validators
- ✅ Web UI fully functional
- ✅ No false alerts
- ✅ Lower resource usage
- ❌ No network-wide analytics

## 🎯 What You Keep vs Lose

### You KEEP (Important Features):
- ✅ All TVC monitoring for YOUR validators
- ✅ Vote batch analysis for YOUR validators
- ✅ Web dashboard with all features
- ✅ Historical TVC data (5 epochs)
- ✅ Performance grading for your validators
- ✅ All API endpoints working
- ✅ Prometheus metrics for your validators

### You LOSE (Network-wide Features):
- ❌ Monitoring ALL validators in network
- ❌ Comparative analysis with other validators
- ❌ Network-wide vote batch analysis
- ❌ Cluster-wide performance metrics

## 🚨 Prevention for Future

When using Solana Exporter configuration flags, understand their scope:

| Flag | Scope | Use Case | Alert Risk |
|------|-------|----------|------------|
| `-nodekey` | Your validators only | Production monitoring | ✅ Safe |
| `-comprehensive-vote-account-tracking` | ALL validators | Research/analytics | ⚠️ Alert risk |
| `-comprehensive-slot-tracking` | ALL validators | Detailed analysis | ⚠️ High resource usage |

## 📝 Quick Reference Commands

```bash
# Stop alerts immediately
sudo systemctl stop solana-exporter

# Check current configuration
sudo systemctl cat solana-exporter

# Apply fixed configuration
curl -s https://raw.githubusercontent.com/noders-team/solana-exporter/master/systemd-service-fixed.service | sudo tee /etc/systemd/system/solana-exporter.service

# Restart with fixed config
sudo systemctl daemon-reload
sudo systemctl start solana-exporter

# Verify fix
curl -s http://localhost:8080/metrics | grep "solana_validator_delinquent" | wc -l
```

## 🎯 Summary

The delinquent alerts issue was caused by the `-comprehensive-vote-account-tracking` flag expanding monitoring scope from "your validators" to "all validators in the network". The recommended solution is to remove this flag, which maintains all important functionality while eliminating false alerts.

**Status after fix**: ✅ Your validator monitoring works perfectly, no more spam alerts from other validators.