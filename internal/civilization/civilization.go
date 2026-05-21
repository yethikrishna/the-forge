// Package civilization provides inter-civilization infrastructure:
// AI org to AI org communication, negotiation, marketplace, reputation,
// federation, identity, procurement, vendor management, and diplomacy.
//
// When one AI org meets another, this is the protocol they speak.
package civilization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ==================== Protocol ====================

// ProtocolVersion is the inter-civilization communication version.
const ProtocolVersion = "1.0.0"

// MessageType defines protocol message types.
type MessageType string

const (
	MsgHello       MessageType = "hello"        // initial handshake
	MsgProposal    MessageType = "proposal"      // propose collaboration/trade
	MsgAccept      MessageType = "accept"        // accept proposal
	MsgReject      MessageType = "reject"        // reject proposal
	MsgQuery       MessageType = "query"         // request information
	MsgResponse    MessageType = "response"      // respond to query
	MsgNotify      MessageType = "notify"        // one-way notification
	MsgNegotiate   MessageType = "negotiate"     // counter-offer
	MsgVerify      MessageType = "verify"        // request proof/verification
	MsgEvidence    MessageType = "evidence"      // provide proof
	MsgTerminate   MessageType = "terminate"     // end relationship
)

// ProtocolMessage is the standard envelope for inter-org communication.
type ProtocolMessage struct {
	Version     string            `json:"version"`
	FromOrg     string            `json:"from_org"`
	ToOrg       string            `json:"to_org"`
	Type        MessageType       `json:"type"`
	Correlation string            `json:"correlation,omitempty"` // for request/response pairing
	Subject     string            `json:"subject"`
	Body        string            `json:"body"`
	Attachments map[string]string `json:"attachments,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Signature   string            `json:"signature,omitempty"` // cryptographic signature
}

// ==================== Identity ====================

// OrgIdentity represents a civilization-level identity for an AI org.
type OrgIdentity struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"public_key,omitempty"`
	Endpoint    string    `json:"endpoint,omitempty"` // communication endpoint
	Capabilities []string `json:"capabilities"`
	FoundedAt   time.Time `json:"founded_at"`
	Version     string    `json:"version"`
	Verified    bool      `json:"verified"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ==================== Reputation ====================

// ReputationEntry tracks one reputation data point.
type ReputationEntry struct {
	ID         string    `json:"id"`
	FromOrg    string    `json:"from_org"`
	ToOrg      string    `json:"to_org"`
	Category   string    `json:"category"` // reliability, quality, speed, honesty
	Score      float64   `json:"score"`    // 0-100
	Details    string    `json:"details,omitempty"`
	Verified   bool      `json:"verified"`
	CreatedAt  time.Time `json:"created_at"`
}

// ReputationScore is an aggregated reputation score.
type ReputationScore struct {
	OrgID       string             `json:"org_id"`
	Overall     float64            `json:"overall"`
	Categories  map[string]float64 `json:"categories"`
	EntryCount  int                `json:"entry_count"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// ==================== Marketplace ====================

// ServiceOffering is a service an org offers on the marketplace.
type ServiceOffering struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // compute, data, research, code, analysis
	PriceUnit   string    `json:"price_unit"` // per_call, per_hour, flat, trade
	PriceAmount float64   `json:"price_amount"`
	Currency    string    `json:"currency"` // credits, usd, trade
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// ServiceRequest is an org looking for a service.
type ServiceRequest struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Budget      float64   `json:"budget,omitempty"`
	Currency    string    `json:"currency,omitempty"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	Status      string    `json:"status"` // open, fulfilled, cancelled
	CreatedAt   time.Time `json:"created_at"`
}

// Transaction records a marketplace transaction.
type Transaction struct {
	ID         string    `json:"id"`
	FromOrg    string    `json:"from_org"`
	ToOrg      string    `json:"to_org"`
	ServiceID  string    `json:"service_id"`
	RequestID  string    `json:"request_id,omitempty"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"` // pending, completed, disputed, refunded
	CreatedAt  time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ==================== Federation ====================

// Federation is a group of allied orgs.
type Federation struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Members     []string  `json:"members"` // org IDs
	Charter     string    `json:"charter,omitempty"` // federation rules
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
}

// FederationProposal is an invitation to join or form a federation.
type FederationProposal struct {
	ID          string    `json:"id"`
	FromOrg     string    `json:"from_org"`
	ToOrg       string    `json:"to_org"`
	FederationID string   `json:"federation_id,omitempty"` // empty = new federation
	Type        string    `json:"type"` // invite, merge, alliance
	Status      string    `json:"status"` // pending, accepted, rejected
	CreatedAt   time.Time `json:"created_at"`
}

// ==================== Diplomacy ====================

// TreatyType categorizes inter-org agreements.
type TreatyType string

const (
	TreatyNonCompete   TreatyType = "non_compete"
	TreatyDataSharing  TreatyType = "data_sharing"
	TreatyMutualDefense TreatyType = "mutual_defense"
	TreatyTrade        TreatyType = "trade"
	TreatyNonAggression TreatyType = "non_aggression"
	TreatyStandards    TreatyType = "standards"
)

// TreatyStatus tracks treaty lifecycle.
type TreatyStatus string

const (
	TreatyProposed  TreatyStatus = "proposed"
	TreatyActive    TreatyStatus = "active"
	TreatyViolated  TreatyStatus = "violated"
	TreatyExpired   TreatyStatus = "expired"
	TreatyCancelled TreatyStatus = "cancelled"
)

// Treaty represents a binding agreement between orgs.
type Treaty struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Type        TreatyType  `json:"type"`
	Parties     []string    `json:"parties"` // org IDs
	Terms       string      `json:"terms"`
	Status      TreatyStatus `json:"status"`
	ProposedBy  string      `json:"proposed_by"`
	ProposedAt  time.Time   `json:"proposed_at"`
	ActivatedAt *time.Time  `json:"activated_at,omitempty"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	Violations  []Violation `json:"violations,omitempty"`
}

// Violation records a treaty violation.
type Violation struct {
	ID          string    `json:"id"`
	TreatyID    string    `json:"treaty_id"`
	ByOrg       string    `json:"by_org"`
	Description string    `json:"description"`
	Evidence    string    `json:"evidence,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Resolved    bool      `json:"resolved"`
}

// ==================== Procurement ====================

// ProcurementRequest is an org buying something.
type ProcurementRequest struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Item        string    `json:"item"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // cloud_credits, api_access, domains, saas
	Quantity    float64   `json:"quantity"`
	MaxPrice    float64   `json:"max_price,omitempty"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"` // draft, submitted, approved, fulfilled, cancelled
	VendorID    string    `json:"vendor_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	FulfilledAt *time.Time `json:"fulfilled_at,omitempty"`
}

// Vendor tracks an external vendor relationship.
type Vendor struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Service     string    `json:"service"`
	SLATier     string    `json:"sla_tier"` // basic, standard, premium
	Status      string    `json:"status"` // active, suspended, terminated
	ContractEnd *time.Time `json:"contract_end,omitempty"`
	Rating      float64   `json:"rating"` // 0-5
	CreatedAt   time.Time `json:"created_at"`
	Notes       string    `json:"notes,omitempty"`
}

// ==================== Hub ====================

// Hub is the main civilization hub — manages all inter-org interactions.
type Hub struct {
	mu          sync.RWMutex
	identity    *OrgIdentity
	peers       map[string]*OrgIdentity     // known peer orgs
	reputation  map[string][]ReputationEntry // org -> entries
	offerings   map[string]*ServiceOffering
	requests    map[string]*ServiceRequest
	transactions map[string]*Transaction
	federations map[string]*Federation
	fedProps    map[string]*FederationProposal
	treaties    map[string]*Treaty
	violations  map[string]*Violation
	procReqs    map[string]*ProcurementRequest
	vendors     map[string]*Vendor
	inbox       map[string]*ProtocolMessage // pending incoming messages
	outbox      map[string]*ProtocolMessage // pending outgoing messages
	path        string
}

// NewHub creates a new civilization hub for this org.
func NewHub(identity *OrgIdentity, persistPath string) *Hub {
	h := &Hub{
		identity:    identity,
		peers:       make(map[string]*OrgIdentity),
		reputation:  make(map[string][]ReputationEntry),
		offerings:   make(map[string]*ServiceOffering),
		requests:    make(map[string]*ServiceRequest),
		transactions: make(map[string]*Transaction),
		federations: make(map[string]*Federation),
		fedProps:    make(map[string]*FederationProposal),
		treaties:    make(map[string]*Treaty),
		violations:  make(map[string]*Violation),
		procReqs:    make(map[string]*ProcurementRequest),
		vendors:     make(map[string]*Vendor),
		inbox:       make(map[string]*ProtocolMessage),
		outbox:      make(map[string]*ProtocolMessage),
		path:        persistPath,
	}
	h.load()
	return h
}

// --- Identity ---

// GetIdentity returns this org's identity.
func (h *Hub) GetIdentity() *OrgIdentity {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.identity
}

// RegisterPeer registers a known peer org.
func (h *Hub) RegisterPeer(peer *OrgIdentity) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers[peer.ID] = peer
	h.persist()
	return nil
}

// ListPeers returns all known peer orgs.
func (h *Hub) ListPeers() []*OrgIdentity {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []*OrgIdentity
	for _, p := range h.peers {
		result = append(result, p)
	}
	return result
}

// --- Protocol Messaging ---

// SendMessage sends a protocol message to another org.
func (h *Hub) SendMessage(toOrg, subject, body string, msgType MessageType) (*ProtocolMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	msg := &ProtocolMessage{
		Version:   ProtocolVersion,
		FromOrg:   h.identity.ID,
		ToOrg:     toOrg,
		Type:      msgType,
		Subject:   subject,
		Body:      body,
		Timestamp: time.Now().UTC(),
	}

	h.outbox[msg.Correlation] = msg
	h.persist()
	return msg, nil
}

// ReceiveMessage receives an incoming protocol message.
func (h *Hub) ReceiveMessage(msg *ProtocolMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if msg.ToOrg != h.identity.ID {
		return fmt.Errorf("message not for this org (got %s, expected %s)", msg.ToOrg, h.identity.ID)
	}

	h.inbox[msg.Correlation] = msg
	h.persist()
	return nil
}

// GetInbox returns pending incoming messages.
func (h *Hub) GetInbox() []*ProtocolMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []*ProtocolMessage
	for _, m := range h.inbox {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

// --- Reputation ---

// AddReputation adds a reputation score for an org.
func (h *Hub) AddReputation(fromOrg, toOrg, category, details string, score float64, verified bool) (*ReputationEntry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := &ReputationEntry{
		ID:        genID("rep"),
		FromOrg:   fromOrg,
		ToOrg:     toOrg,
		Category:  category,
		Score:     score,
		Details:   details,
		Verified:  verified,
		CreatedAt: time.Now().UTC(),
	}

	h.reputation[toOrg] = append(h.reputation[toOrg], *entry)
	h.persist()
	return entry, nil
}

// GetReputation returns aggregated reputation for an org.
func (h *Hub) GetReputation(orgID string) *ReputationScore {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries, ok := h.reputation[orgID]
	if !ok {
		return &ReputationScore{OrgID: orgID}
	}

	categories := make(map[string]float64)
	catCounts := make(map[string]int)
	var total float64

	for _, e := range entries {
		categories[e.Category] += e.Score
		catCounts[e.Category]++
		total += e.Score
	}

	for cat := range categories {
		categories[cat] /= float64(catCounts[cat])
	}

	overall := total / float64(len(entries))

	return &ReputationScore{
		OrgID:      orgID,
		Overall:    overall,
		Categories: categories,
		EntryCount: len(entries),
		UpdatedAt:  time.Now().UTC(),
	}
}

// --- Marketplace ---

// OfferService lists a service on the marketplace.
func (h *Hub) OfferService(name, description, category, priceUnit, currency string, priceAmount float64) (*ServiceOffering, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	offering := &ServiceOffering{
		ID:          genID("svc"),
		OrgID:       h.identity.ID,
		Name:        name,
		Description: description,
		Category:    category,
		PriceUnit:   priceUnit,
		PriceAmount: priceAmount,
		Currency:    currency,
		Active:      true,
		CreatedAt:   time.Now().UTC(),
	}

	h.offerings[offering.ID] = offering
	h.persist()
	return offering, nil
}

// RequestService creates a service request.
func (h *Hub) RequestService(name, description, category, currency string, budget float64, deadline *time.Time) (*ServiceRequest, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	req := &ServiceRequest{
		ID:          genID("req"),
		OrgID:       h.identity.ID,
		Name:        name,
		Description: description,
		Category:    category,
		Budget:      budget,
		Currency:    currency,
		Deadline:    deadline,
		Status:      "open",
		CreatedAt:   time.Now().UTC(),
	}

	h.requests[req.ID] = req
	h.persist()
	return req, nil
}

// RecordTransaction records a marketplace transaction.
func (h *Hub) RecordTransaction(fromOrg, toOrg, serviceID, currency string, amount float64) (*Transaction, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tx := &Transaction{
		ID:        genID("tx"),
		FromOrg:   fromOrg,
		ToOrg:     toOrg,
		ServiceID: serviceID,
		Amount:    amount,
		Currency:  currency,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	h.transactions[tx.ID] = tx
	h.persist()
	return tx, nil
}

// CompleteTransaction marks a transaction as completed.
func (h *Hub) CompleteTransaction(txID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	tx, ok := h.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction %s not found", txID)
	}
	tx.Status = "completed"
	now := time.Now().UTC()
	tx.CompletedAt = &now
	h.persist()
	return nil
}

// ListOfferings returns marketplace offerings.
func (h *Hub) ListOfferings(category string) []*ServiceOffering {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*ServiceOffering
	for _, o := range h.offerings {
		if o.Active && (category == "" || o.Category == category) {
			result = append(result, o)
		}
	}
	return result
}

// --- Federation ---

// CreateFederation creates a new federation.
func (h *Hub) CreateFederation(name, description, charter string, initialMembers []string) (*Federation, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	members := initialMembers
	members = append(members, h.identity.ID)

	fed := &Federation{
		ID:          genID("fed"),
		Name:        name,
		Description: description,
		Members:     members,
		Charter:     charter,
		CreatedAt:   time.Now().UTC(),
		Active:      true,
	}

	h.federations[fed.ID] = fed
	h.persist()
	return fed, nil
}

// InviteToFederation invites an org to join a federation.
func (h *Hub) InviteToFederation(federationID, toOrg string) (*FederationProposal, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	prop := &FederationProposal{
		ID:          genID("fedprop"),
		FromOrg:     h.identity.ID,
		ToOrg:       toOrg,
		FederationID: federationID,
		Type:        "invite",
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
	}

	h.fedProps[prop.ID] = prop
	h.persist()
	return prop, nil
}

// JoinFederation adds an org to a federation (after acceptance).
func (h *Hub) JoinFederation(federationID, orgID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	fed, ok := h.federations[federationID]
	if !ok {
		return fmt.Errorf("federation %s not found", federationID)
	}
	for _, m := range fed.Members {
		if m == orgID {
			return nil // already member
		}
	}
	fed.Members = append(fed.Members, orgID)
	h.persist()
	return nil
}

// ListFederations returns federations this org belongs to.
func (h *Hub) ListFederations() []*Federation {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*Federation
	for _, f := range h.federations {
		if f.Active {
			for _, m := range f.Members {
				if m == h.identity.ID {
					result = append(result, f)
					break
				}
			}
		}
	}
	return result
}

// --- Treaties ---

// ProposeTreaty proposes a treaty to another org.
func (h *Hub) ProposeTreaty(title string, treatyType TreatyType, parties []string, terms string, expiresAt *time.Time) (*Treaty, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	treaty := &Treaty{
		ID:         genID("treaty"),
		Title:      title,
		Type:       treatyType,
		Parties:    parties,
		Terms:      terms,
		Status:     TreatyProposed,
		ProposedBy: h.identity.ID,
		ProposedAt: time.Now().UTC(),
		ExpiresAt:  expiresAt,
	}

	h.treaties[treaty.ID] = treaty
	h.persist()
	return treaty, nil
}

// ActivateTreaty activates a proposed treaty.
func (h *Hub) ActivateTreaty(treatyID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	t, ok := h.treaties[treatyID]
	if !ok {
		return fmt.Errorf("treaty %s not found", treatyID)
	}
	t.Status = TreatyActive
	now := time.Now().UTC()
	t.ActivatedAt = &now
	h.persist()
	return nil
}

// ReportViolation reports a treaty violation.
func (h *Hub) ReportViolation(treatyID, byOrg, description, evidence string) (*Violation, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	t, ok := h.treaties[treatyID]
	if !ok {
		return nil, fmt.Errorf("treaty %s not found", treatyID)
	}

	v := &Violation{
		ID:          genID("viol"),
		TreatyID:    treatyID,
		ByOrg:       byOrg,
		Description: description,
		Evidence:    evidence,
		CreatedAt:   time.Now().UTC(),
	}

	t.Violations = append(t.Violations, *v)
	t.Status = TreatyViolated
	h.violations[v.ID] = v
	h.persist()
	return v, nil
}

// ListTreaties returns treaties filtered by status.
func (h *Hub) ListTreaties(status TreatyStatus) []*Treaty {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*Treaty
	for _, t := range h.treaties {
		if status == "" || t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// --- Procurement ---

// CreateProcurement creates a procurement request.
func (h *Hub) CreateProcurement(item, description, category, currency string, quantity, maxPrice float64) (*ProcurementRequest, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	req := &ProcurementRequest{
		ID:          genID("proc"),
		OrgID:       h.identity.ID,
		Item:        item,
		Description: description,
		Category:    category,
		Quantity:    quantity,
		MaxPrice:    maxPrice,
		Currency:    currency,
		Status:      "draft",
		CreatedAt:   time.Now().UTC(),
	}

	h.procReqs[req.ID] = req
	h.persist()
	return req, nil
}

// ApproveProcurement approves and submits a procurement request.
func (h *Hub) ApproveProcurement(procID, vendorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	p, ok := h.procReqs[procID]
	if !ok {
		return fmt.Errorf("procurement %s not found", procID)
	}
	p.Status = "approved"
	p.VendorID = vendorID
	h.persist()
	return nil
}

// FulfillProcurement marks a procurement as fulfilled.
func (h *Hub) FulfillProcurement(procID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	p, ok := h.procReqs[procID]
	if !ok {
		return fmt.Errorf("procurement %s not found", procID)
	}
	p.Status = "fulfilled"
	now := time.Now().UTC()
	p.FulfilledAt = &now
	h.persist()
	return nil
}

// --- Vendors ---

// AddVendor registers a vendor.
func (h *Hub) AddVendor(name, service, slaTier string, contractEnd *time.Time) (*Vendor, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	v := &Vendor{
		ID:          genID("vendor"),
		Name:        name,
		Service:     service,
		SLATier:     slaTier,
		Status:      "active",
		ContractEnd: contractEnd,
		Rating:      0,
		CreatedAt:   time.Now().UTC(),
	}

	h.vendors[v.ID] = v
	h.persist()
	return v, nil
}

// RateVendor updates a vendor's rating.
func (h *Hub) RateVendor(vendorID string, rating float64) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	v, ok := h.vendors[vendorID]
	if !ok {
		return fmt.Errorf("vendor %s not found", vendorID)
	}
	if rating < 0 || rating > 5 {
		return fmt.Errorf("rating must be 0-5, got %f", rating)
	}
	v.Rating = rating
	h.persist()
	return nil
}

// ListVendors returns active vendors.
func (h *Hub) ListVendors() []*Vendor {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*Vendor
	for _, v := range h.vendors {
		if v.Status == "active" {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Rating > result[j].Rating
	})
	return result
}

// --- Persistence ---

type hubData struct {
	Identity     *OrgIdentity               `json:"identity"`
	Peers        map[string]*OrgIdentity    `json:"peers"`
	Reputation   map[string][]ReputationEntry `json:"reputation"`
	Offerings    map[string]*ServiceOffering `json:"offerings"`
	Requests     map[string]*ServiceRequest  `json:"requests"`
	Transactions map[string]*Transaction     `json:"transactions"`
	Federations  map[string]*Federation      `json:"federations"`
	FedProps     map[string]*FederationProposal `json:"federation_proposals"`
	Treaties     map[string]*Treaty          `json:"treaties"`
	Violations   map[string]*Violation       `json:"violations"`
	ProcReqs     map[string]*ProcurementRequest `json:"procurement_requests"`
	Vendors      map[string]*Vendor          `json:"vendors"`
}

func (h *Hub) persist() {
	if h.path == "" {
		return
	}
	data := hubData{
		Identity:     h.identity,
		Peers:        h.peers,
		Reputation:   h.reputation,
		Offerings:    h.offerings,
		Requests:     h.requests,
		Transactions: h.transactions,
		Federations:  h.federations,
		FedProps:     h.fedProps,
		Treaties:     h.treaties,
		Violations:   h.violations,
		ProcReqs:     h.procReqs,
		Vendors:      h.vendors,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(h.path), 0755)
	os.WriteFile(h.path, raw, 0644)
}

func (h *Hub) load() {
	if h.path == "" {
		return
	}
	raw, err := os.ReadFile(h.path)
	if err != nil {
		return
	}
	var data hubData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	if data.Peers != nil {
		h.peers = data.Peers
	}
	if data.Reputation != nil {
		h.reputation = data.Reputation
	}
	if data.Offerings != nil {
		h.offerings = data.Offerings
	}
	if data.Requests != nil {
		h.requests = data.Requests
	}
	if data.Transactions != nil {
		h.transactions = data.Transactions
	}
	if data.Federations != nil {
		h.federations = data.Federations
	}
	if data.FedProps != nil {
		h.fedProps = data.FedProps
	}
	if data.Treaties != nil {
		h.treaties = data.Treaties
	}
	if data.Violations != nil {
		h.violations = data.Violations
	}
	if data.ProcReqs != nil {
		h.procReqs = data.ProcReqs
	}
	if data.Vendors != nil {
		h.vendors = data.Vendors
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
