#!/bin/bash

# Log Viewer Script for Shopify ERP Middleware
# Usage: ./view_logs.sh [options]

set -e

# Configuration
LOG_DIR="${LOG_DIR:-./logs}"
TODAY=$(date +%Y-%m-%d)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Helper functions
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_usage() {
    echo "📊 Log Viewer for Shopify ERP Middleware"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -t, --tail              Tail all log files (live view)"
    echo "  -r, --requests [ID]     Show all logs for a specific request ID"
    echo "  -w, --webhooks          Show today's incoming webhooks"
    echo "  -s, --soap              Show today's outgoing SOAP requests"
    echo "  -e, --errors            Show only error responses"
    echo "  -c, --count             Show count of requests by type"
    echo "  -d, --date [YYYY-MM-DD] Show logs for specific date (default: today)"
    echo ""
    echo "Examples:"
    echo "  $0 --tail                   # Live view of all logs"
    echo "  $0 --requests a1b2c3d4      # Show all logs for request ID"
    echo "  $0 --webhooks               # Show today's webhooks"
    echo "  $0 --errors                 # Show only failed requests"
    echo "  $0 --date 2024-01-15        # Show logs for specific date"
}

show_file_with_header() {
    local file="$1"
    local header="$2"
    
    if [[ -f "$file" ]]; then
        print_header "$header"
        if command -v jq &> /dev/null; then
            # Pretty print JSON if jq is available
            cat "$file" | jq -r '. | @json' | jq .
        else
            cat "$file"
        fi
        echo ""
    else
        echo -e "${YELLOW}⚠️  $file not found${NC}"
    fi
}

filter_by_request_id() {
    local request_id="$1"
    local date="$2"
    
    print_header "All logs for Request ID: $request_id (Date: $date)"
    
    local files=(
        "$LOG_DIR/${date}_incoming_webhook.log"
        "$LOG_DIR/${date}_outgoing_soap.log"
        "$LOG_DIR/${date}_soap_response.log"
    )
    
    local found=false
    for file in "${files[@]}"; do
        if [[ -f "$file" ]]; then
            if command -v jq &> /dev/null; then
                local matches=$(jq -r "select(.request_id == \"$request_id\")" "$file" 2>/dev/null || echo "")
                if [[ -n "$matches" ]]; then
                    echo -e "${GREEN}📄 $(basename "$file"):${NC}"
                    echo "$matches" | jq .
                    echo ""
                    found=true
                fi
            else
                local matches=$(grep "\"request_id\": \"$request_id\"" "$file" || echo "")
                if [[ -n "$matches" ]]; then
                    echo -e "${GREEN}📄 $(basename "$file"):${NC}"
                    echo "$matches"
                    echo ""
                    found=true
                fi
            fi
        fi
    done
    
    if [[ "$found" == false ]]; then
        echo -e "${YELLOW}⚠️  No logs found for request ID: $request_id${NC}"
    fi
}

show_errors() {
    local date="$1"
    print_header "Error Responses (Date: $date)"
    
    local response_file="$LOG_DIR/${date}_soap_response.log"
    
    if [[ -f "$response_file" ]]; then
        if command -v jq &> /dev/null; then
            jq -r 'select(.status_code >= 400 or .error != null)' "$response_file" 2>/dev/null | jq .
        else
            grep -E '"status_code": [4-5][0-9][0-9]|"error":' "$response_file" || echo "No errors found"
        fi
    else
        echo -e "${YELLOW}⚠️  No response log file found for $date${NC}"
    fi
}

show_count() {
    local date="$1"
    print_header "Request Count Summary (Date: $date)"
    
    local files=(
        "$LOG_DIR/${date}_incoming_webhook.log"
        "$LOG_DIR/${date}_outgoing_soap.log"
        "$LOG_DIR/${date}_soap_response.log"
    )
    
    for file in "${files[@]}"; do
        if [[ -f "$file" ]]; then
            local count=$(wc -l < "$file")
            local type=$(basename "$file" .log | cut -d'_' -f2-)
            echo -e "${GREEN}📊 $type: $count requests${NC}"
        fi
    done
    
    echo ""
    
    # Show success/error breakdown
    local response_file="$LOG_DIR/${date}_soap_response.log"
    if [[ -f "$response_file" ]] && command -v jq &> /dev/null; then
        local success=$(jq -r 'select(.status_code >= 200 and .status_code < 300)' "$response_file" 2>/dev/null | wc -l)
        local errors=$(jq -r 'select(.status_code >= 400 or .error != null)' "$response_file" 2>/dev/null | wc -l)
        echo -e "${GREEN}✅ Successful: $success${NC}"
        echo -e "${RED}❌ Errors: $errors${NC}"
    fi
}

tail_logs() {
    print_header "Live Log Tail (Press Ctrl+C to stop)"
    
    local files=()
    for file in "$LOG_DIR"/*_*.log; do
        if [[ -f "$file" ]]; then
            files+=("$file")
        fi
    done
    
    if [[ ${#files[@]} -eq 0 ]]; then
        echo -e "${YELLOW}⚠️  No log files found in $LOG_DIR${NC}"
        exit 1
    fi
    
    tail -f "${files[@]}"
}

# Main script logic
main() {
    local date="$TODAY"
    
    case "${1:-}" in
        -h|--help)
            print_usage
            exit 0
            ;;
        -t|--tail)
            tail_logs
            ;;
        -r|--requests)
            if [[ -z "${2:-}" ]]; then
                echo -e "${RED}❌ Error: Request ID required${NC}"
                echo "Usage: $0 --requests <request_id>"
                exit 1
            fi
            filter_by_request_id "$2" "$date"
            ;;
        -w|--webhooks)
            show_file_with_header "$LOG_DIR/${date}_incoming_webhook.log" "Incoming Webhooks (Date: $date)"
            ;;
        -s|--soap)
            show_file_with_header "$LOG_DIR/${date}_outgoing_soap.log" "Outgoing SOAP Requests (Date: $date)"
            show_file_with_header "$LOG_DIR/${date}_soap_response.log" "SOAP Responses (Date: $date)"
            ;;
        -e|--errors)
            show_errors "$date"
            ;;
        -c|--count)
            show_count "$date"
            ;;
        -d|--date)
            if [[ -z "${2:-}" ]]; then
                echo -e "${RED}❌ Error: Date required in YYYY-MM-DD format${NC}"
                exit 1
            fi
            date="$2"
            show_count "$date"
            ;;
        "")
            # Default: show today's summary
            echo -e "${PURPLE}📊 Shopify ERP Middleware - Log Summary${NC}"
            echo -e "${PURPLE}Date: $date${NC}"
            echo -e "${PURPLE}Log Directory: $LOG_DIR${NC}"
            echo ""
            show_count "$date"
            echo ""
            echo "💡 Use $0 --help for more options"
            ;;
        *)
            echo -e "${RED}❌ Unknown option: $1${NC}"
            print_usage
            exit 1
            ;;
    esac
}

# Check if log directory exists
if [[ ! -d "$LOG_DIR" ]]; then
    echo -e "${YELLOW}⚠️  Log directory $LOG_DIR does not exist${NC}"
    echo "Make sure the service has been started and processed some requests."
    exit 1
fi

# Run main function
main "$@"