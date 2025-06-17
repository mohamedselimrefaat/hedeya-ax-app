package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultERPEndpoint = "https://httpbin.org/post" // Temporary test endpoint that accepts any request
	MaxRetries         = 3
	RetryDelay         = 2 * time.Second
	SOAPAction         = "http://tempuri.org/CreateOrder" // Update this to match your AX service
	DefaultLogDir      = "./logs"
)

// LogEntry represents a log entry for requests/responses
type LogEntry struct {
	RequestID   string      `json:"request_id"`
	Timestamp   string      `json:"timestamp"`
	Type        string      `json:"type"` // "incoming_webhook", "outgoing_soap", "soap_response"
	Method      string      `json:"method,omitempty"`
	URL         string      `json:"url,omitempty"`
	Headers     interface{} `json:"headers,omitempty"`
	Body        interface{} `json:"body,omitempty"`
	StatusCode  int         `json:"status_code,omitempty"`
	Error       string      `json:"error,omitempty"`
	OrderID     string      `json:"order_id,omitempty"`
}

// Logger handles file-based logging
type Logger struct {
	logDir string
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = DefaultLogDir
	}
	
	// Create log directory if it doesn't exist (but files will be ephemeral on App Platform)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Warning: Could not create log directory %s: %v", logDir, err)
		logDir = "." // Fall back to current directory
	}
	
	// Log the logging strategy
	if os.Getenv("DIGITAL_OCEAN_APP") != "" {
		log.Printf("Running on DigitalOcean App Platform - logs will appear in Runtime Logs")
	} else {
		log.Printf("Running locally - logs saved to %s/", logDir)
	}
	
	return &Logger{
		logDir: logDir,
	}
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// writeLogEntry writes a log entry to the appropriate file
func (l *Logger) writeLogEntry(entry LogEntry) {
	// Generate filename based on date and type
	now := time.Now()
	filename := fmt.Sprintf("%s_%s.log", 
		now.Format("2006-01-02"), 
		entry.Type)
	
	filepath := filepath.Join(l.logDir, filename)
	
	// Convert entry to JSON
	jsonData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		log.Printf("Error marshaling log entry: %v", err)
		return
	}
	
	// ALSO OUTPUT TO CONSOLE for DigitalOcean Runtime Logs
	log.Printf("LOG_ENTRY[%s]: %s", entry.Type, string(jsonData))
	
	// Write to file (will be ephemeral on App Platform)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Error opening log file %s: %v", filepath, err)
		return
	}
	defer file.Close()
	
	// Write entry with newline separator
	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		log.Printf("Error writing to log file: %v", err)
	}
}

// LogIncomingWebhook logs incoming Shopify webhook requests
func (l *Logger) LogIncomingWebhook(requestID string, headers http.Header, body []byte, orderID string) {
	// Parse body as JSON for better formatting
	var bodyJSON interface{}
	if err := json.Unmarshal(body, &bodyJSON); err != nil {
		bodyJSON = string(body) // Fall back to string if not valid JSON
	}
	
	entry := LogEntry{
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Type:      "incoming_webhook",
		Method:    "POST",
		URL:       "/webhook",
		Headers:   headers,
		Body:      bodyJSON,
		OrderID:   orderID,
	}
	
	l.writeLogEntry(entry)
}

// LogOutgoingSOAP logs outgoing SOAP requests to ERP
func (l *Logger) LogOutgoingSOAP(requestID string, url string, headers http.Header, soapBody string, orderID string) {
	entry := LogEntry{
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Type:      "outgoing_soap",
		Method:    "POST",
		URL:       url,
		Headers:   headers,
		Body:      soapBody,
		OrderID:   orderID,
	}
	
	l.writeLogEntry(entry)
}

// LogSOAPResponse logs responses from ERP SOAP service
func (l *Logger) LogSOAPResponse(requestID string, statusCode int, headers http.Header, responseBody string, orderID string, err error) {
	entry := LogEntry{
		RequestID:  requestID,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Type:       "soap_response",
		StatusCode: statusCode,
		Headers:    headers,
		Body:       responseBody,
		OrderID:    orderID,
	}
	
	if err != nil {
		entry.Error = err.Error()
	}
	
	l.writeLogEntry(entry)
}

// ShopifyOrder represents the structure of a Shopify order
type ShopifyOrder struct {
	ID                int64    `json:"id"`
	OrderNumber       int      `json:"order_number"`
	Email             string   `json:"email"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	TotalPrice        string   `json:"total_price"`
	SubtotalPrice     string   `json:"subtotal_price"`
	TotalTax          string   `json:"total_tax"`
	Currency          string   `json:"currency"`
	FinancialStatus   string   `json:"financial_status"`
	FulfillmentStatus string   `json:"fulfillment_status"`
	Customer          Customer `json:"customer"`
	LineItems         []LineItem `json:"line_items"`
	ShippingAddress   Address    `json:"shipping_address"`
	BillingAddress    Address    `json:"billing_address"`
}

type Customer struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type LineItem struct {
	ID              int64  `json:"id"`
	ProductID       int64  `json:"product_id"`
	VariantID       int64  `json:"variant_id"`
	Title           string `json:"title"`
	Name            string `json:"name"`
	Quantity        int    `json:"quantity"`
	Price           string `json:"price"`
	SKU             string `json:"sku"`
	VariantTitle    string `json:"variant_title"`
	FulfillmentService string `json:"fulfillment_service"`
}

type Address struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Company      string `json:"company"`
	Address1     string `json:"address1"`
	Address2     string `json:"address2"`
	City         string `json:"city"`
	Province     string `json:"province"`
	Country      string `json:"country"`
	Zip          string `json:"zip"`
	Phone        string `json:"phone"`
	ProvinceCode string `json:"province_code"`
	CountryCode  string `json:"country_code"`
}

// ERPOrder represents the transformed order structure for ERP system
type ERPOrder struct {
	OrderID           string      `json:"order_id"`
	OrderNumber       string      `json:"order_number"`
	CustomerEmail     string      `json:"customer_email"`
	CustomerName      string      `json:"customer_name"`
	CustomerPhone     string      `json:"customer_phone"`
	OrderDate         string      `json:"order_date"`
	TotalAmount       string      `json:"total_amount"`
	SubtotalAmount    string      `json:"subtotal_amount"`
	TaxAmount         string      `json:"tax_amount"`
	Currency          string      `json:"currency"`
	PaymentStatus     string      `json:"payment_status"`
	FulfillmentStatus string      `json:"fulfillment_status"`
	Items             []ERPItem   `json:"items"`
	ShippingAddress   ERPAddress  `json:"shipping_address"`
	BillingAddress    ERPAddress  `json:"billing_address"`
	Timestamp         string      `json:"timestamp"`
}

type ERPItem struct {
	SKU          string `json:"sku"`
	ProductName  string `json:"product_name"`
	Quantity     int    `json:"quantity"`
	UnitPrice    string `json:"unit_price"`
	VariantTitle string `json:"variant_title"`
}

type ERPAddress struct {
	Name         string `json:"name"`
	Company      string `json:"company"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	Phone        string `json:"phone"`
}

// Server represents our HTTP server
type Server struct {
	httpClient *http.Client
	logger     *Logger
}

// NewServer creates a new server instance
func NewServer() *Server {
	return &Server{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: NewLogger(),
	}
}

// xmlEscape escapes XML special characters
func xmlEscape(s string) string {
	return html.EscapeString(s)
}

// createSOAPEnvelope creates a SOAP XML envelope for the ERP order
func (s *Server) createSOAPEnvelope(erpOrder *ERPOrder) string {
	// Create SOAP envelope with the order data
	// Update the namespace and method name according to your AX 2012 service WSDL
	soapEnvelope := `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" 
               xmlns:tem="http://tempuri.org/">
  <soap:Header/>
  <soap:Body>
    <tem:CreateOrder>
      <tem:order>
        <tem:OrderID>` + xmlEscape(erpOrder.OrderID) + `</tem:OrderID>
        <tem:OrderNumber>` + xmlEscape(erpOrder.OrderNumber) + `</tem:OrderNumber>
        <tem:CustomerEmail>` + xmlEscape(erpOrder.CustomerEmail) + `</tem:CustomerEmail>
        <tem:CustomerName>` + xmlEscape(erpOrder.CustomerName) + `</tem:CustomerName>
        <tem:CustomerPhone>` + xmlEscape(erpOrder.CustomerPhone) + `</tem:CustomerPhone>
        <tem:OrderDate>` + xmlEscape(erpOrder.OrderDate) + `</tem:OrderDate>
        <tem:TotalAmount>` + xmlEscape(erpOrder.TotalAmount) + `</tem:TotalAmount>
        <tem:SubtotalAmount>` + xmlEscape(erpOrder.SubtotalAmount) + `</tem:SubtotalAmount>
        <tem:TaxAmount>` + xmlEscape(erpOrder.TaxAmount) + `</tem:TaxAmount>
        <tem:Currency>` + xmlEscape(erpOrder.Currency) + `</tem:Currency>
        <tem:PaymentStatus>` + xmlEscape(erpOrder.PaymentStatus) + `</tem:PaymentStatus>
        <tem:FulfillmentStatus>` + xmlEscape(erpOrder.FulfillmentStatus) + `</tem:FulfillmentStatus>
        <tem:ShippingAddress>
          <tem:Name>` + xmlEscape(erpOrder.ShippingAddress.Name) + `</tem:Name>
          <tem:Company>` + xmlEscape(erpOrder.ShippingAddress.Company) + `</tem:Company>
          <tem:AddressLine1>` + xmlEscape(erpOrder.ShippingAddress.AddressLine1) + `</tem:AddressLine1>
          <tem:AddressLine2>` + xmlEscape(erpOrder.ShippingAddress.AddressLine2) + `</tem:AddressLine2>
          <tem:City>` + xmlEscape(erpOrder.ShippingAddress.City) + `</tem:City>
          <tem:State>` + xmlEscape(erpOrder.ShippingAddress.State) + `</tem:State>
          <tem:PostalCode>` + xmlEscape(erpOrder.ShippingAddress.PostalCode) + `</tem:PostalCode>
          <tem:Country>` + xmlEscape(erpOrder.ShippingAddress.Country) + `</tem:Country>
          <tem:Phone>` + xmlEscape(erpOrder.ShippingAddress.Phone) + `</tem:Phone>
        </tem:ShippingAddress>
        <tem:BillingAddress>
          <tem:Name>` + xmlEscape(erpOrder.BillingAddress.Name) + `</tem:Name>
          <tem:Company>` + xmlEscape(erpOrder.BillingAddress.Company) + `</tem:Company>
          <tem:AddressLine1>` + xmlEscape(erpOrder.BillingAddress.AddressLine1) + `</tem:AddressLine1>
          <tem:AddressLine2>` + xmlEscape(erpOrder.BillingAddress.AddressLine2) + `</tem:AddressLine2>
          <tem:City>` + xmlEscape(erpOrder.BillingAddress.City) + `</tem:City>
          <tem:State>` + xmlEscape(erpOrder.BillingAddress.State) + `</tem:State>
          <tem:PostalCode>` + xmlEscape(erpOrder.BillingAddress.PostalCode) + `</tem:PostalCode>
          <tem:Country>` + xmlEscape(erpOrder.BillingAddress.Country) + `</tem:Country>
          <tem:Phone>` + xmlEscape(erpOrder.BillingAddress.Phone) + `</tem:Phone>
        </tem:BillingAddress>
        <tem:Items>`

	// Add line items
	for _, item := range erpOrder.Items {
		soapEnvelope += `
          <tem:Item>
            <tem:SKU>` + xmlEscape(item.SKU) + `</tem:SKU>
            <tem:ProductName>` + xmlEscape(item.ProductName) + `</tem:ProductName>
            <tem:Quantity>` + fmt.Sprintf("%d", item.Quantity) + `</tem:Quantity>
            <tem:UnitPrice>` + xmlEscape(item.UnitPrice) + `</tem:UnitPrice>
            <tem:VariantTitle>` + xmlEscape(item.VariantTitle) + `</tem:VariantTitle>
          </tem:Item>`
	}

	soapEnvelope += `
        </tem:Items>
        <tem:Timestamp>` + xmlEscape(erpOrder.Timestamp) + `</tem:Timestamp>
      </tem:order>
    </tem:CreateOrder>
  </soap:Body>
</soap:Envelope>`

	return soapEnvelope
}

// transformOrder converts Shopify order to ERP format
func (s *Server) transformOrder(shopifyOrder *ShopifyOrder) *ERPOrder {
	// Transform line items
	items := make([]ERPItem, len(shopifyOrder.LineItems))
	for i, item := range shopifyOrder.LineItems {
		items[i] = ERPItem{
			SKU:          item.SKU,
			ProductName:  item.Title,
			Quantity:     item.Quantity,
			UnitPrice:    item.Price,
			VariantTitle: item.VariantTitle,
		}
	}

	// Transform addresses
	shippingAddr := ERPAddress{
		Name:         fmt.Sprintf("%s %s", shopifyOrder.ShippingAddress.FirstName, shopifyOrder.ShippingAddress.LastName),
		Company:      shopifyOrder.ShippingAddress.Company,
		AddressLine1: shopifyOrder.ShippingAddress.Address1,
		AddressLine2: shopifyOrder.ShippingAddress.Address2,
		City:         shopifyOrder.ShippingAddress.City,
		State:        shopifyOrder.ShippingAddress.Province,
		PostalCode:   shopifyOrder.ShippingAddress.Zip,
		Country:      shopifyOrder.ShippingAddress.Country,
		Phone:        shopifyOrder.ShippingAddress.Phone,
	}

	billingAddr := ERPAddress{
		Name:         fmt.Sprintf("%s %s", shopifyOrder.BillingAddress.FirstName, shopifyOrder.BillingAddress.LastName),
		Company:      shopifyOrder.BillingAddress.Company,
		AddressLine1: shopifyOrder.BillingAddress.Address1,
		AddressLine2: shopifyOrder.BillingAddress.Address2,
		City:         shopifyOrder.BillingAddress.City,
		State:        shopifyOrder.BillingAddress.Province,
		PostalCode:   shopifyOrder.BillingAddress.Zip,
		Country:      shopifyOrder.BillingAddress.Country,
		Phone:        shopifyOrder.BillingAddress.Phone,
	}

	return &ERPOrder{
		OrderID:           fmt.Sprintf("%d", shopifyOrder.ID),
		OrderNumber:       fmt.Sprintf("%d", shopifyOrder.OrderNumber),
		CustomerEmail:     shopifyOrder.Email,
		CustomerName:      fmt.Sprintf("%s %s", shopifyOrder.Customer.FirstName, shopifyOrder.Customer.LastName),
		CustomerPhone:     shopifyOrder.Customer.Phone,
		OrderDate:         shopifyOrder.CreatedAt,
		TotalAmount:       shopifyOrder.TotalPrice,
		SubtotalAmount:    shopifyOrder.SubtotalPrice,
		TaxAmount:         shopifyOrder.TotalTax,
		Currency:          shopifyOrder.Currency,
		PaymentStatus:     shopifyOrder.FinancialStatus,
		FulfillmentStatus: shopifyOrder.FulfillmentStatus,
		Items:             items,
		ShippingAddress:   shippingAddr,
		BillingAddress:    billingAddr,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
	}
}

// sendToERP sends the transformed order to the ERP system with retry logic
func (s *Server) sendToERP(erpOrder *ERPOrder, requestID string) error {
	// Get ERP endpoint from environment variable or use default
	erpEndpoint := os.Getenv("ERP_ENDPOINT")
	if erpEndpoint == "" {
		erpEndpoint = DefaultERPEndpoint
	}

	// Get SOAP Action from environment variable or use default
	soapAction := os.Getenv("SOAP_ACTION")
	if soapAction == "" {
		soapAction = SOAPAction
	}

	// Create SOAP XML envelope
	soapXML := s.createSOAPEnvelope(erpOrder)

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		req, err := http.NewRequest("POST", erpEndpoint, bytes.NewBufferString(soapXML))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set SOAP headers
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
		req.Header.Set("SOAPAction", fmt.Sprintf(`"%s"`, soapAction))
		req.Header.Set("User-Agent", "Shopify-ERP-Middleware/1.0")

		// Log outgoing SOAP request
		s.logger.LogOutgoingSOAP(requestID, erpEndpoint, req.Header, soapXML, erpOrder.OrderID)
		
		log.Printf("[%s] Sending SOAP request to %s (attempt %d)", requestID, erpEndpoint, attempt)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			log.Printf("[%s] Attempt %d failed: %v", requestID, attempt, err)
			s.logger.LogSOAPResponse(requestID, 0, nil, "", erpOrder.OrderID, err)
			
			if attempt < MaxRetries {
				time.Sleep(RetryDelay * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("failed to send request after %d attempts: %w", MaxRetries, err)
		}

		defer resp.Body.Close()

		// Read response body
		responseBody, _ := io.ReadAll(resp.Body)
		responseStr := string(responseBody)
		
		// Log SOAP response
		s.logger.LogSOAPResponse(requestID, resp.StatusCode, resp.Header, responseStr, erpOrder.OrderID, nil)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[%s] Successfully sent order %s to ERP (attempt %d)", requestID, erpOrder.OrderID, attempt)
			log.Printf("[%s] ERP response: %s", requestID, responseStr)
			return nil
		}

		log.Printf("[%s] Attempt %d failed with status %d: %s", requestID, attempt, resp.StatusCode, responseStr)

		if attempt < MaxRetries {
			time.Sleep(RetryDelay * time.Duration(attempt))
		}
	}

	return fmt.Errorf("failed to send order to ERP after %d attempts", MaxRetries)
}

// handleWebhook handles incoming Shopify webhooks
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Generate unique request ID for tracking
	requestID := generateRequestID()
	
	if r.Method != http.MethodPost {
		log.Printf("[%s] Method not allowed: %s", requestID, r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[%s] Error reading request body: %v", requestID, err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Log the webhook topic for debugging
	webhookTopic := r.Header.Get("X-Shopify-Topic")
	log.Printf("[%s] Received webhook: %s", requestID, webhookTopic)

	// Parse the Shopify order
	var shopifyOrder ShopifyOrder
	if err := json.Unmarshal(body, &shopifyOrder); err != nil {
		log.Printf("[%s] Error parsing Shopify order: %v", requestID, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	orderID := fmt.Sprintf("%d", shopifyOrder.ID)
	log.Printf("[%s] Processing order ID: %d, Order Number: %d", requestID, shopifyOrder.ID, shopifyOrder.OrderNumber)

	// Log incoming webhook
	s.logger.LogIncomingWebhook(requestID, r.Header, body, orderID)

	// Transform the order for ERP
	erpOrder := s.transformOrder(&shopifyOrder)

	// Send to ERP system
	if err := s.sendToERP(erpOrder, requestID); err != nil {
		log.Printf("[%s] Error sending order to ERP: %v", requestID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Respond with success
	response := map[string]string{
		"status":     "success",
		"order_id":   orderID,
		"request_id": requestID,
		"message":    "Order successfully sent to ERP",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	
	log.Printf("[%s] Successfully processed order %s", requestID, orderID)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "shopify-erp-middleware",
	})
}

// handleRoot handles root path requests
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"service":     "Shopify to ERP Middleware",
		"version":     "1.0.0",
		"description": "Middleware service to forward Shopify orders to Microsoft Dynamics AX 2012",
		"endpoints": "/webhook (POST) - Shopify webhook handler, /health (GET) - Health check",
	})
}

func main() {
	server := NewServer()

	// Set up routes
	http.HandleFunc("/", server.handleRoot)
	http.HandleFunc("/webhook", server.handleWebhook)
	http.HandleFunc("/health", server.handleHealth)

	// Get port from environment variable (DigitalOcean App Platform requirement)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get ERP endpoint
	erpEndpoint := os.Getenv("ERP_ENDPOINT")
	if erpEndpoint == "" {
		erpEndpoint = DefaultERPEndpoint
	}

	// Get SOAP Action
	soapAction := os.Getenv("SOAP_ACTION")
	if soapAction == "" {
		soapAction = SOAPAction
	}

	// Get log directory
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = DefaultLogDir
	}

	log.Printf("Starting Shopify to Microsoft Dynamics AX 2012 Middleware")
	log.Printf("Server port: %s", port)
	log.Printf("Webhook endpoint: /webhook")
	log.Printf("Health check endpoint: /health")
	log.Printf("ERP endpoint: %s", erpEndpoint)
	log.Printf("SOAP Action: %s", soapAction)
	log.Printf("Log directory: %s", logDir)
	log.Printf("Log files:")
	log.Printf("  - Incoming webhooks: %s/YYYY-MM-DD_incoming_webhook.log", logDir)
	log.Printf("  - Outgoing SOAP: %s/YYYY-MM-DD_outgoing_soap.log", logDir)
	log.Printf("  - SOAP responses: %s/YYYY-MM-DD_soap_response.log", logDir)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}