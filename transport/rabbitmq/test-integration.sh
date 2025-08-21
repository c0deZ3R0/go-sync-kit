#!/bin/bash

# RabbitMQ Integration Test Runner
# This script sets up RabbitMQ via Docker Compose and runs integration tests

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.test.yml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
    success "Docker is running"
}

# Function to check if docker-compose is available
check_compose() {
    if command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    elif docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        error "Neither 'docker-compose' nor 'docker compose' is available"
        exit 1
    fi
    success "Using $COMPOSE_CMD"
}

# Function to start RabbitMQ services
start_services() {
    log "Starting RabbitMQ services..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" up -d rabbitmq
    
    log "Waiting for RabbitMQ to be healthy..."
    local max_attempts=60
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if $COMPOSE_CMD -f "$COMPOSE_FILE" ps rabbitmq | grep -q "healthy"; then
            success "RabbitMQ is healthy"
            return 0
        fi
        
        if [ $((attempt % 10)) -eq 0 ]; then
            log "Still waiting for RabbitMQ... (attempt $attempt/$max_attempts)"
        fi
        
        sleep 2
        attempt=$((attempt + 1))
    done
    
    error "RabbitMQ failed to become healthy within $(($max_attempts * 2)) seconds"
    show_rabbitmq_logs
    return 1
}

# Function to show RabbitMQ logs
show_rabbitmq_logs() {
    log "RabbitMQ logs:"
    $COMPOSE_CMD -f "$COMPOSE_FILE" logs rabbitmq
}

# Function to run tests
run_tests() {
    log "Running integration tests..."
    
    # Run tests in the container with proper environment
    $COMPOSE_CMD -f "$COMPOSE_FILE" run --rm test-runner sh -c "
        cd /app && \
        go mod download && \
        go test -v -tags=integration ./transport/rabbitmq -run TestIntegration
    "
}

# Function to run tests locally (if RabbitMQ is available)
run_tests_local() {
    log "Running integration tests locally..."
    
    export RABBITMQ_URL="amqp://synckit_user:synckit_pass@localhost:5672/"
    
    cd "$PROJECT_ROOT"
    go test -v -tags=integration ./transport/rabbitmq -run TestIntegration
}

# Function to stop services
stop_services() {
    log "Stopping services..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" down
    success "Services stopped"
}

# Function to clean up everything
cleanup() {
    log "Cleaning up..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" down -v --remove-orphans
    success "Cleanup complete"
}

# Function to show management UI info
show_management_info() {
    echo
    log "RabbitMQ Management UI is available at:"
    echo "  URL: http://localhost:15672"
    echo "  Username: synckit_user"
    echo "  Password: synckit_pass"
    echo
}

# Function to show usage
usage() {
    echo "Usage: $0 [COMMAND]"
    echo
    echo "Commands:"
    echo "  test        Start services and run integration tests (default)"
    echo "  test-local  Run tests against locally running RabbitMQ"
    echo "  start       Start RabbitMQ services only"
    echo "  stop        Stop services"
    echo "  cleanup     Stop services and remove volumes"
    echo "  logs        Show RabbitMQ logs"
    echo "  shell       Open shell in test container"
    echo "  help        Show this help"
    echo
}

# Main execution
main() {
    local command="${1:-test}"
    
    case "$command" in
        test)
            check_docker
            check_compose
            
            # Trap to ensure cleanup on exit
            trap 'stop_services' EXIT
            
            start_services
            show_management_info
            run_tests
            success "Integration tests completed successfully"
            ;;
            
        test-local)
            run_tests_local
            ;;
            
        start)
            check_docker
            check_compose
            start_services
            show_management_info
            log "RabbitMQ is running. Use '$0 stop' to stop services."
            ;;
            
        stop)
            check_compose
            stop_services
            ;;
            
        cleanup)
            check_compose
            cleanup
            ;;
            
        logs)
            check_compose
            show_rabbitmq_logs
            ;;
            
        shell)
            check_compose
            log "Opening shell in test container..."
            $COMPOSE_CMD -f "$COMPOSE_FILE" run --rm test-runner sh
            ;;
            
        help)
            usage
            ;;
            
        *)
            error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"
