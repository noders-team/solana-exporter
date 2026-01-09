# Solana Exporter Web UI Guide

## 🎯 Overview

The Solana Exporter now includes a **built-in web dashboard** that provides real-time monitoring of your Solana validator with a focus on Vote Credits (TVC) analysis and vote batch monitoring. No external tools required!

## 🚀 Quick Start

1. **Start Solana Exporter with your validator keys:**
```bash
./solana-exporter \
  -nodekey YOUR_VALIDATOR_IDENTITY \
  -votekey YOUR_VALIDATOR_VOTE_ACCOUNT \
  -rpc-url http://localhost:8899 \
  -listen-address :8080
```

2. **Open your browser and navigate to:**
```
http://localhost:8080
```

3. **That's it!** Your dashboard is now live with real-time metrics.

## 📊 Dashboard Features

### 🚨 Critical Status Section
- **Node Health** - Real-time health indicator with slots behind cluster
- **Validator Status** - Active/delinquent status with stake information  
- **Vote Performance** - TVC performance percentage with vote lag
- **Current Epoch** - Epoch number and completion progress

### 🔍 Vote Batch Analysis Table
Detailed breakdown of recent vote batches showing:
- **Batch ID** - Sequential batch number
- **Start Time** - When the batch was processed
- **Slot Range** - Covered slot range (e.g., "392257999-392259999")
- **Avg Latency** - Average voting response time in seconds
- **Missed TVCs** - Vote credits lost in this batch
- **Missed Slots** - Slots not voted on
- **Performance** - Batch success rate percentage

### 📈 Network Activity
- **Current Slot** - Latest processed slot
- **Block Height** - Current block number
- **Transactions** - Total network transactions

## 🌐 Available Endpoints

### Web Interface
- **`/`** - Main dashboard (Web UI)
- **`/health`** - Simple health check page
- **`/metrics`** - Prometheus metrics (existing)

### API Endpoints
- **`/api/dashboard`** - JSON dashboard data
- **`/api/health`** - Health status JSON
- **`/api/vote-batches`** - Vote batch analysis data
- **`/api/metrics/live`** - Live metrics endpoint

## 📱 User Interface

### Design Features
- **Dark Theme** - Modern GitHub-style dark interface
- **Responsive Design** - Works on desktop, tablet, and mobile
- **Real-time Updates** - Auto-refreshes every 15 seconds
- **Color-coded Status** - Green (healthy), yellow (warning), red (critical)
- **Clean Typography** - Optimized for readability

### Status Indicators
- 🟢 **Green** - Healthy status (>95% vote performance, node healthy)
- 🟡 **Yellow** - Warning status (90-95% performance, minor issues)
- 🔴 **Red** - Critical status (<90% performance, node unhealthy, delinquent)

## ⚙️ Configuration

### Access Configuration
The web UI is automatically enabled when you start Solana Exporter. Configure access via:

```bash
# Custom listen address
./solana-exporter -listen-address :3000

# Access at http://localhost:3000
```

### Multi-Validator Support
When monitoring multiple validators, the UI shows data for the first configured validator:

```bash
./solana-exporter \
  -nodekey VALIDATOR_1 -nodekey VALIDATOR_2 \
  -votekey VOTEKEY_1 -votekey VOTEKEY_2 \
  -comprehensive-vote-account-tracking
```

## 📊 Metrics Integration

The web UI pulls data from the same metrics that Prometheus uses:

### Critical Metrics Displayed
- `solana_node_is_healthy` - Node health status
- `solana_validator_delinquent` - Validator delinquency
- `solana_validator_credits_missed_pct` - Vote credit miss rate
- `solana_vote_batch_performance_pct` - Batch-level performance
- `solana_vote_batch_avg_latency_seconds` - Voting latency
- `solana_node_num_slots_behind` - Sync status

### Real-time Updates
- Dashboard data refreshes every **15 seconds**
- Vote batch data refreshes on demand via "Refresh" button
- Auto-updates timestamp shown in header

## 🔧 Troubleshooting

### Common Issues

**1. Dashboard shows "Loading..." indefinitely**
- Check that your validator keys are correctly configured
- Verify RPC endpoint is accessible
- Check browser console for JavaScript errors

**2. Vote batch table shows "No batch data available"**
- Ensure validator is actively voting
- Vote account must have recent voting history
- Try refreshing batch data manually

**3. UI not accessible**
- Verify listen address is correct
- Check firewall settings
- Ensure port is not in use by another service

### Health Check
Visit `/health` for a simple status page to verify the service is running:
```
http://localhost:8080/health
```

## 🎯 Performance Impact

### Resource Usage
- **Minimal overhead** - Web UI adds ~5% CPU usage
- **Memory efficient** - Dashboard data cached for 15 seconds
- **Network friendly** - Lightweight JSON API responses

### Production Considerations
- Web UI is suitable for production use
- No authentication built-in (add reverse proxy if needed)
- Dashboard data includes sensitive validator information
- Consider firewall rules for external access

## 🔗 Integration Examples

### Reverse Proxy (Nginx)
```nginx
server {
    listen 80;
    server_name solana-monitor.example.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Docker Deployment
```dockerfile
FROM your-solana-exporter-image

EXPOSE 8080
CMD ["./solana-exporter", "-listen-address", ":8080", "-nodekey", "$VALIDATOR_NODEKEY"]
```

### Kubernetes Service
```yaml
apiVersion: v1
kind: Service
metadata:
  name: solana-exporter-ui
spec:
  selector:
    app: solana-exporter
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

## 🎨 Customization

### Styling
The UI uses CSS custom properties for easy theming. Main color variables:
```css
:root {
    --bg-primary: #0d1117;
    --text-primary: #c9d1d9;
    --success-color: #238636;
    --warning-color: #d29922;
    --error-color: #da3633;
}
```

### Adding Custom Metrics
Extend the dashboard by modifying the API endpoints in `web_ui.go`:

1. Add new data fields to `DashboardData` struct
2. Implement collection logic in `collectDashboardData()`
3. Update HTML template and JavaScript to display new metrics

## 📚 Advanced Features

### API Usage
Fetch dashboard data programmatically:

```bash
# Get current dashboard data
curl http://localhost:8080/api/dashboard

# Get vote batch analysis
curl http://localhost:8080/api/vote-batches

# Simple health check
curl http://localhost:8080/api/health
```

### Webhook Integration
Use API endpoints for external monitoring:

```python
import requests
import json

# Check validator health
response = requests.get('http://localhost:8080/api/dashboard')
data = response.json()

if not data['node_health']['is_healthy']:
    send_alert(f"Validator unhealthy: {data['node_health']['slots_behind']} slots behind")

if data['vote_credits']['missed_percent'] > 5:
    send_alert(f"High TVC miss rate: {data['vote_credits']['missed_percent']}%")
```

## 🎯 Best Practices

### Monitoring Strategy
1. **Primary Dashboard** - Use web UI for real-time monitoring
2. **Historical Analysis** - Use Prometheus + Grafana for trends
3. **Alerting** - Combine both web UI API and Prometheus alerts
4. **Mobile Access** - Web UI is mobile-responsive for on-the-go monitoring

### Security
- **Internal Networks** - Keep web UI on private networks
- **Authentication** - Add auth proxy for external access
- **HTTPS** - Use reverse proxy with SSL for production
- **Rate Limiting** - Consider API rate limits for public deployments

## 📞 Support

### Getting Help
- **Issues** - Check GitHub issues for common problems
- **Documentation** - Refer to main README.md for configuration
- **Metrics Reference** - See METRICS_SUMMARY.md for complete metric list

### Contributing
The web UI is built with:
- **Backend** - Go HTTP server with embedded assets
- **Frontend** - Vanilla JavaScript + CSS (no frameworks)
- **Styling** - GitHub-inspired dark theme
- **APIs** - RESTful JSON endpoints

---

**The Solana Exporter Web UI provides the fastest way to monitor your validator's Vote Credits performance and overall health - all built-in with zero external dependencies!** 🚀