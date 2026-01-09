#!/bin/bash

# Solana Exporter Installation and Update Script
# Version: 2.0.0
# Author: Noders Team
# Repository: https://github.com/noders-team/solana-exporter

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_URL="https://github.com/noders-team/solana-exporter"
SERVICE_NAME="solana-exporter"
INSTALL_DIR="/home/solana/solana-exporter"
USER="solana"
GROUP="solana"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check if running as root or with sudo
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root or with sudo"
        exit 1
    fi

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go 1.22 or later"
        exit 1
    fi

    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.22"
    if ! printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V -C; then
        log_error "Go version $GO_VERSION is too old. Minimum required: $REQUIRED_VERSION"
        exit 1
    fi

    # Check if git is installed
    if ! command -v git &> /dev/null; then
        log_error "Git is not installed. Please install git"
        exit 1
    fi

    # Check if systemctl is available
    if ! command -v systemctl &> /dev/null; then
        log_error "systemctl is not available. This script requires systemd"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

create_user() {
    if ! id "$USER" &>/dev/null; then
        log_info "Creating user $USER..."
        useradd -r -m -s /bin/bash -d "/home/$USER" "$USER"
        log_success "User $USER created"
    else
        log_info "User $USER already exists"
    fi
}

stop_service() {
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "Stopping $SERVICE_NAME service..."
        systemctl stop "$SERVICE_NAME"
        log_success "Service stopped"
    fi
}

backup_config() {
    if [[ -f "/home/$USER/.env" ]]; then
        log_info "Backing up existing configuration..."
        cp "/home/$USER/.env" "/home/$USER/.env.backup.$(date +%Y%m%d_%H%M%S)"
        log_success "Configuration backed up"
    fi
}

install_or_update() {
    log_info "Installing/updating Solana Exporter..."

    # Create install directory
    mkdir -p "$INSTALL_DIR"

    if [[ -d "$INSTALL_DIR/.git" ]]; then
        log_info "Updating existing installation..."
        cd "$INSTALL_DIR"
        sudo -u "$USER" git fetch origin
        sudo -u "$USER" git reset --hard origin/master
    else
        log_info "Fresh installation..."
        if [[ -d "$INSTALL_DIR" ]]; then
            rm -rf "$INSTALL_DIR"
        fi
        sudo -u "$USER" git clone "$REPO_URL" "$INSTALL_DIR"
        cd "$INSTALL_DIR"
    fi

    # Build the application
    log_info "Building Solana Exporter..."
    sudo -u "$USER" go mod tidy
    sudo -u "$USER" go build -o solana-exporter ./cmd/solana-exporter

    # Make binary executable
    chmod +x solana-exporter

    log_success "Build completed successfully"
}

setup_systemd_service() {
    log_info "Setting up systemd service..."

    # Create systemd service file
    cat > "/etc/systemd/system/$SERVICE_NAME.service" << EOF
[Unit]
Description=Solana Exporter with Web UI
Documentation=https://github.com/noders-team/solana-exporter
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR

# Environment configuration
EnvironmentFile=/home/$USER/.env

# Enhanced startup command with Web UI support
# NOTE: comprehensive-vote-account-tracking removed to prevent alert spam
# Add it manually only if you need network-wide analytics
ExecStart=$INSTALL_DIR/solana-exporter \\
  -nodekey "\$NODEKEY" \\
  -votekey "\$VOTEKEY" \\
  -active-identity "\$ACTIVE_IDENTITY" \\
  -balance-address "\$BALANCE_ADDRESS" \\
  -monitor-block-sizes \\
  -rpc-url "\$RPC_URL" \\
  -listen-address "\${LISTEN_ADDRESS:-:8080}" \\
  -reference-rpc-url "\${REFERENCE_RPC_URL:-https://api.mainnet-beta.solana.com}"

# Service management
Restart=always
RestartSec=5s
TimeoutStartSec=60s
TimeoutStopSec=30s

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$INSTALL_DIR
ReadOnlyPaths=/home/$USER/.env

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=solana-exporter

# Process management
KillMode=mixed
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
EOF

    # Set ownership
    chown -R "$USER:$GROUP" "$INSTALL_DIR"

    # Reload systemd
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"

    log_success "Systemd service configured"
}

setup_environment() {
    ENV_FILE="/home/$USER/.env"

    if [[ ! -f "$ENV_FILE" ]]; then
        log_info "Creating environment configuration..."

        # Copy template
        sudo -u "$USER" cp "$INSTALL_DIR/env-template" "$ENV_FILE"

        # Set secure permissions
        chown "$USER:$GROUP" "$ENV_FILE"
        chmod 600 "$ENV_FILE"

        log_warning "Please edit $ENV_FILE with your validator configuration"
        log_warning "Required variables: NODEKEY, VOTEKEY, RPC_URL"
    else
        log_info "Environment file already exists at $ENV_FILE"
    fi
}

setup_firewall() {
    if command -v ufw &> /dev/null; then
        log_info "Configuring UFW firewall..."
        ufw allow 8080/tcp comment "Solana Exporter Web UI"
        log_success "Firewall rule added for port 8080"
    elif command -v firewall-cmd &> /dev/null; then
        log_info "Configuring firewalld..."
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --reload
        log_success "Firewall rule added for port 8080"
    else
        log_warning "No firewall detected. Make sure port 8080 is accessible if needed"
    fi
}

show_status() {
    log_info "Installation completed! Here's the status:"
    echo
    echo "📁 Installation Directory: $INSTALL_DIR"
    echo "👤 Service User: $USER"
    echo "🔧 Service Name: $SERVICE_NAME"
    echo "📋 Config File: /home/$USER/.env"
    echo
    echo "🌐 Web UI will be available at: http://your-server:8080"
    echo "📊 Prometheus metrics: http://your-server:8080/metrics"
    echo "💚 Health check: http://your-server:8080/health"
    echo
    echo "⚠️  IMPORTANT: Service configured for YOUR validators only"
    echo "   To monitor ALL network validators, add: -comprehensive-vote-account-tracking"
    echo "   (Warning: this will create alerts for ALL delinquent validators in network)"
    echo
    echo "📋 Next steps:"
    echo "1. Edit /home/$USER/.env with your validator keys"
    echo "2. Start the service: sudo systemctl start $SERVICE_NAME"
    echo "3. Check status: sudo systemctl status $SERVICE_NAME"
    echo "4. View logs: sudo journalctl -u $SERVICE_NAME -f"
    echo "5. Access Web UI: http://localhost:8080"
    echo

    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        log_success "Service is enabled and will start on boot"
    fi

    if [[ -f "/home/$USER/.env.backup."* ]]; then
        log_info "Previous configuration backed up to /home/$USER/.env.backup.*"
    fi
}

start_service() {
    if [[ "$1" == "--start" ]]; then
        log_info "Starting $SERVICE_NAME service..."
        if systemctl start "$SERVICE_NAME"; then
            log_success "Service started successfully"
            sleep 2
            if systemctl is-active --quiet "$SERVICE_NAME"; then
                log_success "Service is running"
                # Test Web UI
                if curl -s http://localhost:8080/health > /dev/null 2>&1; then
                    log_success "Web UI is accessible at http://localhost:8080"
                else
                    log_warning "Web UI might not be ready yet, check service logs"
                fi
            else
                log_error "Service failed to start, check logs: journalctl -u $SERVICE_NAME"
            fi
        else
            log_error "Failed to start service"
        fi
    else
        log_info "Service not started. Use --start flag to start automatically"
        log_info "To start manually: sudo systemctl start $SERVICE_NAME"
    fi
}

show_help() {
    echo "Solana Exporter Installation Script"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --start       Start the service after installation"
    echo "  --help        Show this help message"
    echo
    echo "Examples:"
    echo "  $0                    # Install only"
    echo "  $0 --start           # Install and start service"
    echo
    echo "Features:"
    echo "  • Built-in Web Dashboard at http://localhost:8080"
    echo "  • Comprehensive Vote Credits (TVC) monitoring"
    echo "  • Vote batch analysis with latency tracking"
    echo "  • 32+ Prometheus metrics"
    echo "  • Real-time performance monitoring"
    echo "  • Mobile-responsive interface"
    echo
}

# Main execution
main() {
    case "${1:-}" in
        --help|-h)
            show_help
            exit 0
            ;;
    esac

    log_info "🚀 Solana Exporter Installation Script v2.0.0"
    log_info "Repository: $REPO_URL"
    echo

    check_prerequisites
    create_user
    stop_service
    backup_config
    install_or_update
    setup_systemd_service
    setup_environment
    setup_firewall
    show_status
    start_service "$@"

    echo
    log_success "🎉 Installation completed successfully!"
    log_info "Don't forget to configure your .env file and start the service!"
}

# Run main function with all arguments
main "$@"
