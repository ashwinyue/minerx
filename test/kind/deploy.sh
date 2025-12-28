#!/bin/bash
# MinerX Kind Test Deployment Script

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to create a test Chain
create_chain() {
    local chain_name="${1:-test-chain}"
    local namespace="${2:-default}"
    local miner_type="${3:-small}"

    log_info "Creating Chain: ${chain_name} in namespace ${namespace}"

    cat <<EOF | kubectl apply -f -
apiVersion: apps.onex.io/v1alpha1
kind: Chain
metadata:
  name: ${chain_name}
  namespace: ${namespace}
spec:
  displayName: Test Chain
  minerType: ${miner_type}
  image: nginx:alpine
  minMineIntervalSeconds: 43200
EOF

    kubectl wait --for=condition=ready chain ${chain_name} -n ${namespace} --timeout=2m || true
    log_info "Chain created: ${chain_name}"
}

# Function to create a test Miner
create_miner() {
    local miner_name="${1:-test-miner}"
    local namespace="${2:-default}"
    local chain_name="${3:-test-chain}"
    local miner_type="${4:-medium}"

    log_info "Creating Miner: ${miner_name} in namespace ${namespace}"

    cat <<EOF | kubectl apply -f -
apiVersion: apps.onex.io/v1alpha1
kind: Miner
metadata:
  name: ${miner_name}
  namespace: ${namespace}
spec:
  displayName: Test Miner
  minerType: ${miner_type}
  chainName: ${chain_name}
  restartPolicy: Always
EOF

    log_info "Miner created: ${miner_name}"
}

# Function to create a test MinerSet
create_minerset() {
    local minerset_name="${1:-test-minerset}"
    local namespace="${2:-default}"
    local chain_name="${3:-test-chain}"
    local replicas="${4:-3}"
    local delete_policy="${5:-Random}"

    log_info "Creating MinerSet: ${minerset_name} in namespace ${namespace}"

    cat <<EOF | kubectl apply -f -
apiVersion: apps.onex.io/v1alpha1
kind: MinerSet
metadata:
  name: ${minerset_name}
  namespace: ${namespace}
spec:
  displayName: Test MinerSet
  replicas: ${replicas}
  deletePolicy: ${delete_policy}
  selector:
    matchLabels:
      app: miner
  template:
    metadata:
      labels:
        app: miner
    spec:
      displayName: Template Miner
      minerType: medium
      chainName: ${chain_name}
      restartPolicy: Always
EOF

    log_info "MinerSet created: ${minerset_name}"
}

# Function to delete all test resources
cleanup_resources() {
    local namespace="${1:-default}"

    log_info "Cleaning up test resources in namespace: ${namespace}"

    kubectl delete minerset --all -n ${namespace} --ignore-not-found=true
    kubectl delete miner --all -n ${namespace} --ignore-not-found=true
    kubectl delete chain --all -n ${namespace} --ignore-not-found=true

    log_info "Cleanup completed."
}

# Function to show resource status
show_status() {
    local namespace="${1:-default}"

    echo ""
    log_info "=== Resource Status in ${namespace} ==="
    echo ""

    echo "Chains:"
    kubectl get chain -n ${namespace} 2>/dev/null || echo "  No Chains found"
    echo ""

    echo "Miners:"
    kubectl get miner -n ${namespace} 2>/dev/null || echo "  No Miners found"
    echo ""

    echo "MinerSets:"
    kubectl get minerset -n ${namespace} 2>/dev/null || echo "  No MinerSets found"
    echo ""

    echo "Pods:"
    kubectl get pod -n ${namespace} -l app=miner 2>/dev/null || echo "  No Miner Pods found"
    echo ""
}

# Function to run full test scenario
run_test_scenario() {
    local namespace="${1:-test-minerx}"

    log_info "=== Running Full Test Scenario ==="
    echo ""

    # Cleanup first
    cleanup_resources ${namespace}

    # Step 1: Create Chain
    log_info "Step 1: Creating Chain..."
    create_chain genesis-chain ${namespace} small
    sleep 3
    show_status ${namespace}

    # Step 2: Create MinerSet
    log_info "Step 2: Creating MinerSet..."
    create_minerset test-minerset ${namespace} genesis-chain 3 Random
    sleep 5
    show_status ${namespace}

    # Step 3: Scale up
    log_info "Step 3: Scaling up MinerSet to 5 replicas..."
    kubectl patch minerset test-minerset -n ${namespace} --type='merge' -p='{"spec":{"replicas":5}}'
    sleep 5
    show_status ${namespace}

    # Step 4: Scale down
    log_info "Step 4: Scaling down MinerSet to 2 replicas..."
    kubectl patch minerset test-minerset -n ${namespace} --type='merge' -p='{"spec":{"replicas":2}}'
    sleep 5
    show_status ${namespace}

    # Step 5: Create independent Miner
    log_info "Step 5: Creating independent Miner..."
    create_miner independent-miner ${namespace} genesis-chain large
    sleep 3
    show_status ${namespace}

    # Final status
    log_info "=== Test Scenario Completed ==="
    show_status ${namespace}

    log_info "Check logs for detailed information:"
    echo "  kubectl logs -n minerx-system -l control-plane=controller-manager -f"
    echo "  kubectl describe chain genesis-chain -n ${namespace}"
    echo "  kubectl describe minerset test-minerset -n ${namespace}"
}

# Print usage
print_usage() {
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  setup           Setup Kind test environment (requires: setup.sh)"
    echo "  chain <name>   Create a Chain"
    echo "  miner <name>    Create a Miner"
    echo "  minerset <name> Create a MinerSet"
    echo "  status          Show resource status"
    echo "  scenario        Run full test scenario"
    echo "  cleanup         Delete all test resources"
    echo ""
    echo "Examples:"
    echo "  $0 chain test-chain"
    echo "  $0 miner test-miner"
    echo "  $0 minerset test-minerset"
    echo "  $0 status"
    echo "  $0 scenario"
    echo "  $0 cleanup"
}

# Main execution
main() {
    if [ $# -lt 1 ]; then
        print_usage
        exit 1
    fi

    local command="$1"
    shift

    case "$command" in
        setup)
            log_info "Setup is done by setup.sh. Please run 'cd ${SCRIPT_DIR} && ./setup.sh' first."
            ;;
        chain)
            create_chain "$@"
            ;;
        miner)
            create_miner "$@"
            ;;
        minerset)
            create_minerset "$@"
            ;;
        status)
            show_status "$@"
            ;;
        scenario)
            run_test_scenario "$@"
            ;;
        cleanup)
            cleanup_resources "$@"
            ;;
        *)
            log_error "Unknown command: $command"
            print_usage
            exit 1
            ;;
    esac
}

main "$@"
