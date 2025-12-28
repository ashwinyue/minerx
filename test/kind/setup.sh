#!/bin/bash
# MinerX Kind Test Environment Setup Script

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Kind cluster configuration
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-minerx-test}"
KIND_CONFIG="${SCRIPT_DIR}/kind-config.yaml"
KIND_VERSION="${KIND_VERSION:-v0.20.0}"

# Function to print colored messages
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kind is installed
check_kind() {
    if ! command -v kind &> /dev/null; then
        log_error "kind is not installed. Please install kind from https://kind.sigs.k8s.io/"
        log_info "Installation: curl -Lo ./kind https://kind.sigs.k8s.io/downloads/${KIND_VERSION}/kind-$(uname)-amd64"
        log_info "              chmod +x ./kind && sudo mv ./kind /usr/local/bin/"
        exit 1
    fi

    local kind_version=$(kind version | awk '{print $2}')
    log_info "kind version: ${kind_version}"
}

# Check if Docker is running
check_docker() {
    if ! docker info &> /dev/null; then
        log_error "Docker is not running. Please start Docker."
        exit 1
    fi
    log_info "Docker is running"
}

# Check if cluster already exists
check_cluster() {
    if kind get clusters | grep -q "${KIND_CLUSTER_NAME}"; then
        log_warn "Kind cluster '${KIND_CLUSTER_NAME}' already exists."
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Deleting existing cluster..."
            kind delete cluster --name "${KIND_CLUSTER_NAME}"
        else
            log_info "Using existing cluster."
            return 0
        fi
    fi
}

# Create Kind cluster
create_cluster() {
    log_info "Creating Kind cluster '${KIND_CLUSTER_NAME}'..."
    if kind create cluster --config="${KIND_CONFIG}" --name="${KIND_CLUSTER_NAME}"; then
        log_info "Kind cluster created successfully."
    else
        log_error "Failed to create Kind cluster."
        exit 1
    fi
}

# Set kubectl context
set_kubectl_context() {
    log_info "Setting kubectl context..."
    kubectl cluster-info --context="kind-${KIND_CLUSTER_NAME}"
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"

    if kubectl get nodes &> /dev/null; then
        log_info "Kubectl context set successfully."
        log_info "Cluster nodes:"
        kubectl get nodes
    else
        log_error "Failed to connect to cluster."
        exit 1
    fi
}

# Build minerx manager image
build_manager_image() {
    log_info "Building minerx manager Docker image..."

    cd "${PROJECT_ROOT}"

    # Build the manager binary
    if ! make build; then
        log_error "Failed to build manager binary."
        exit 1
    fi

    # Build Docker image
    local IMG="minerx/manager:latest"
    if docker build -t "${IMG}" .; then
        log_info "Manager image built successfully: ${IMG}"
    else
        log_error "Failed to build manager image."
        exit 1
    fi
}

# Load image into Kind cluster
load_manager_image() {
    log_info "Loading manager image into Kind cluster..."
    local IMG="minerx/manager:latest"

    if kind load docker-image "${IMG}" --name="${KIND_CLUSTER_NAME}"; then
        log_info "Image loaded successfully."
    else
        log_error "Failed to load image."
        exit 1
    fi
}

# Install CRDs
install_crds() {
    log_info "Installing CRDs..."
    cd "${PROJECT_ROOT}"

    if kubectl apply -f config/crd/bases/; then
        log_info "CRDs installed successfully."
    else
        log_error "Failed to install CRDs."
        exit 1
    fi

    # Wait for CRDs to be established
    log_info "Waiting for CRDs to be ready..."
    sleep 5
}

# Deploy manager
deploy_manager() {
    log_info "Deploying minerx manager..."
    cd "${PROJECT_ROOT}/config/manager"

    # Update image in kustomization
    kustomize edit set image controller=minerx/manager:latest

    # Apply manifests
    if kubectl apply -k .; then
        log_info "Manager deployed successfully."
    else
        log_error "Failed to deploy manager."
        exit 1
    fi

    # Wait for manager pod to be ready
    log_info "Waiting for manager pod to be ready..."
    kubectl wait --for=condition=ready pod -l control-plane=controller-manager -n minerx-system --timeout=3m

    log_info "Manager is ready."
    kubectl get pods -n minerx-system
}

# Create test namespaces
create_namespaces() {
    log_info "Creating test namespaces..."
    kubectl create namespace default --dry-run=client 2>/dev/null || true
    kubectl create namespace test-minerx --dry-run=client 2>/dev/null || kubectl create namespace test-minerx
    log_info "Namespaces ready."
}

# Print cluster info
print_cluster_info() {
    echo ""
    log_info "=== Kind Cluster Info ==="
    echo ""
    kind get clusters
    echo ""
    kubectl cluster-info
    echo ""
    log_info "=== Nodes ==="
    kubectl get nodes
    echo ""
    log_info "=== Namespaces ==="
    kubectl get namespaces
    echo ""
    log_info "=== CRDs ==="
    kubectl get crd | grep onex.io || echo "No CRDs found"
    echo ""
    log_info "=== Test Environment Ready ==="
}

# Main execution
main() {
    echo ""
    log_info "=== MinerX Kind Test Environment Setup ==="
    echo ""

    check_kind
    check_docker
    check_cluster
    create_cluster
    set_kubectl_context
    build_manager_image
    load_manager_image
    install_crds
    deploy_manager
    create_namespaces
    print_cluster_info

    echo ""
    log_info "Setup completed successfully!"
    log_info "You can now run tests with: make test-e2e"
    echo ""
}

# Cleanup function
cleanup() {
    if [ $? -ne 0 ]; then
        log_error "Setup failed. Please check logs above."
    fi
}

# Trap exit signals
trap cleanup EXIT

# Run main
main "$@"
