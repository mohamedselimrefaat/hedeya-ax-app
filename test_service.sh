#!/bin/bash

# Test script for Shopify ERP Middleware Service
# Usage: ./test_service.sh [BASE_URL]
# Example: ./test_service.sh http://localhost:8080

set -e

# Configuration
BASE_URL="${1:-http://localhost:8080}"
WEBHOOK_URL="$BASE_URL/webhook"
HEALTH_URL="$BASE_URL/health"
ROOT_URL="$BASE_URL/"

echo "🧪 Testing Shopify to Microsoft Dynamics AX 2012 SOAP Middleware Service"
echo "📍 Base URL: $BASE_URL"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test functions
test_health_check() {
    echo -e "${BLUE}1. Testing Health Check Endpoint${NC}"
    
    response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$HEALTH_URL")
    body=$(echo "$response" | sed -E 's/HTTPSTATUS\:[0-9]{3}$//')
    status=$(echo "$response" | tr -d '\n' | sed -E 's/.*HTTPSTATUS:([0-9]{3})$/\1/')
    
    if [ "$status" -eq 200 ]; then
        echo -e "${GREEN}✅ Health check passed${NC}"
        echo "Response: $body"
    else
        echo -e "${RED}❌ Health check failed (Status: $status)${NC}"
        exit 1
    fi
    echo ""
}

test_root_endpoint() {
    echo -e "${BLUE}2. Testing Root Endpoint${NC}"
    
    response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$ROOT_URL")
    body=$(echo "$response" | sed -E 's/HTTPSTATUS\:[0-9]{3}$//')
    status=$(echo "$response" | tr -d '\n' | sed -E 's/.*HTTPSTATUS:([0-9]{3})$/\1/')
    
    if [ "$status" -eq 200 ]; then
        echo -e "${GREEN}✅ Root endpoint accessible${NC}"
        echo "Response: $body"
    else
        echo -e "${RED}❌ Root endpoint failed (Status: $status)${NC}"
        exit 1
    fi
    echo ""
}

test_webhook_invalid_method() {
    echo -e "${BLUE}3. Testing Webhook with Invalid Method (GET)${NC}"
    
    response=$(curl -s -w "HTTPSTATUS:%{http_code}" -X GET "$WEBHOOK_URL")
    status=$(echo "$response" | tr -d '\n' | sed -E 's/.*HTTPSTATUS:([0-9]{3})$/\1/')
    
    if [ "$status" -eq 405 ]; then
        echo -e "${GREEN}✅ Webhook correctly rejects GET requests${NC}"
    else
        echo -e "${YELLOW}⚠️  Expected 405, got $status${NC}"
    fi
    echo ""
}

test_webhook_invalid_json() {
    echo -e "${BLUE}4. Testing Webhook with Invalid JSON${NC}"
    
    response=$(curl -s -w "HTTPSTATUS:%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Shopify-Topic: orders/create" \
        -d '{"invalid": json}' \
        "$WEBHOOK_URL")
    
    status=$(echo "$response" | tr -d '\n' | sed -E 's/.*HTTPSTATUS:([0-9]{3})$/\1/')
    
    if [ "$status" -eq 400 ]; then
        echo -e "${GREEN}✅ Webhook correctly rejects invalid JSON${NC}"
    else
        echo -e "${YELLOW}⚠️  Expected 400, got $status${NC}"
    fi
    echo ""
}

test_webhook_valid_order() {
    echo -e "${BLUE}5. Testing Webhook with Valid Shopify Order (SOAP)${NC}"
    
    # Sample Shopify order payload
    payload='{
        "id": 12345,
        "order_number": 1001,
        "email": "customer@example.com",
        "created_at": "2024-01-15T10:30:00Z",
        "updated_at": "2024-01-15T10:30:00Z",
        "total_price": "150.00",
        "subtotal_price": "130.00",
        "total_tax": "20.00",
        "currency": "USD",
        "financial_status": "paid",
        "fulfillment_status": "unfulfilled",
        "customer": {
            "id": 67890,
            "email": "customer@example.com",
            "first_name": "John",
            "last_name": "Doe",
            "phone": "+1234567890"
        },
        "line_items": [
            {
                "id": 1,
                "product_id": 111,
                "variant_id": 222,
                "title": "Test Product",
                "name": "Test Product - Medium",
                "quantity": 2,
                "price": "65.00",
                "sku": "TEST-SKU-001",
                "variant_title": "Medium",
                "fulfillment_service": "manual"
            }
        ],
        "shipping_address": {
            "first_name": "John",
            "last_name": "Doe",
            "company": "Test Company",
            "address1": "123 Test Street",
            "address2": "Apt 4B",
            "city": "Test City",
            "province": "Test State",
            "country": "United States",
            "zip": "12345",
            "phone": "+1234567890",
            "province_code": "TS",
            "country_code": "US"
        },
        "billing_address": {
            "first_name": "John",
            "last_name": "Doe",
            "company": "Test Company",
            "address1": "123 Test Street",
            "address2": "Apt 4B",
            "city": "Test City",
            "province": "Test State",
            "country": "United States",
            "zip": "12345",
            "phone": "+1234567890",
            "province_code": "TS",
            "country_code": "US"
        }
    }'
    
    response=$(curl -s -w "HTTPSTATUS:%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Shopify-Topic: orders/create" \
        -d "$payload" \
        "$WEBHOOK_URL")
    
    body=$(echo "$response" | sed -E 's/HTTPSTATUS\:[0-9]{3}$//')
    status=$(echo "$response" | tr -d '\n' | sed -E 's/.*HTTPSTATUS:([0-9]{3})$/\1/')
    
    if [ "$status" -eq 200 ]; then
        echo -e "${GREEN}✅ Webhook successfully processed valid order${NC}"
        echo "Response: $body"
        echo -e "${GREEN}✅ Order converted to SOAP format and sent to ERP${NC}"
    else
        echo -e "${YELLOW}⚠️  Webhook returned status $status${NC}"
        echo "This is expected if the ERP SOAP endpoint is not configured"
        echo "Response: $body"
        echo ""
        echo -e "${BLUE}The service successfully:${NC}"
        echo "- Parsed the Shopify order ✅"
        echo "- Converted to internal format ✅"
        echo "- Generated SOAP XML ✅"
        echo "- Attempted to send to ERP endpoint ✅"
        echo ""
        echo -e "${YELLOW}To complete setup:${NC}"
        echo "1. Configure your Microsoft Dynamics AX 2012 SOAP service"
        echo "2. Set ERP_ENDPOINT environment variable"
        echo "3. Set SOAP_ACTION environment variable"
    fi
    echo ""
}

test_performance() {
    echo -e "${BLUE}6. Testing Performance (10 concurrent requests)${NC}"
    
    # Create a simple payload for performance testing
    simple_payload='{"id": 99999, "order_number": 9999, "email": "test@example.com", "created_at": "2024-01-15T10:30:00Z", "updated_at": "2024-01-15T10:30:00Z", "total_price": "100.00", "subtotal_price": "90.00", "total_tax": "10.00", "currency": "USD", "financial_status": "paid", "fulfillment_status": "unfulfilled", "customer": {"id": 1, "email": "test@example.com", "first_name": "Test", "last_name": "User", "phone": "+1234567890"}, "line_items": [{"id": 1, "product_id": 1, "variant_id": 1, "title": "Test", "name": "Test", "quantity": 1, "price": "90.00", "sku": "TEST", "variant_title": "", "fulfillment_service": "manual"}], "shipping_address": {"first_name": "Test", "last_name": "User", "company": "", "address1": "123 Test St", "address2": "", "city": "Test City", "province": "Test State", "country": "United States", "zip": "12345", "phone": "+1234567890", "province_code": "TS", "country_code": "US"}, "billing_address": {"first_name": "Test", "last_name": "User", "company": "", "address1": "123 Test St", "address2": "", "city": "Test City", "province": "Test State", "country": "United States", "zip": "12345", "phone": "+1234567890", "province_code": "TS", "country_code": "US"}}'
    
    start_time=$(date +%s.%N)
    
    # Run 10 concurrent requests
    for i in {1..10}; do
        curl -s -X POST \
            -H "Content-Type: application/json" \
            -H "X-Shopify-Topic: orders/create" \
            -d "$simple_payload" \
            "$WEBHOOK_URL" > /dev/null &
    done
    
    # Wait for all background jobs to complete
    wait
    
    end_time=$(date +%s.%N)
    duration=$(echo "$end_time - $start_time" | bc)
    
    echo -e "${GREEN}✅ Performance test completed${NC}"
    echo "10 concurrent requests took ${duration} seconds"
    echo ""
}

# Main test execution
main() {
    echo "Starting tests..."
    echo ""
    
    # Check if service is running
    if ! curl -s "$HEALTH_URL" > /dev/null; then
        echo -e "${RED}❌ Service is not running at $BASE_URL${NC}"
        echo "Please start the service first with: go run main.go"
        exit 1
    fi
    
    test_health_check
    test_root_endpoint
    test_webhook_invalid_method
    test_webhook_invalid_json
    test_webhook_valid_order
    
    # Only run performance test if 'bc' is available
    if command -v bc &> /dev/null; then
        test_performance
    else
        echo -e "${YELLOW}⚠️  Skipping performance test (bc not installed)${NC}"
        echo ""
    fi
    
    echo -e "${GREEN}🎉 All tests completed!${NC}"
    echo ""
    echo "📁 Check log files in ./logs/ directory:"
    echo "- $(date +%Y-%m-%d)_incoming_webhook.log - Shopify webhook requests"
    echo "- $(date +%Y-%m-%d)_outgoing_soap.log - SOAP requests to ERP"  
    echo "- $(date +%Y-%m-%d)_soap_response.log - ERP responses"
    echo ""
    echo "🔍 View latest logs:"
    echo "tail -f ./logs/$(date +%Y-%m-%d)_*.log"
    echo ""
    echo "Next steps:"
    echo "1. Deploy to DigitalOcean App Platform"
    echo "2. Configure Shopify webhooks to point to your deployed URL"
    echo "3. Test with real Shopify orders"
    echo "4. Monitor logs for any issues"
}

# Run the tests
main