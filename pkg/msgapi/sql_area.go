package msgapi

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/askovpen/gossiped/pkg/config"
	"github.com/askovpen/gossiped/pkg/database"
	"github.com/askovpen/gossiped/pkg/types"
	"github.com/askovpen/gossiped/pkg/utils"
	"gorm.io/gorm"
)

// DateHelper for time conversions
var dateHelper = database.DateHelper{}

// Global cache for message counts
var (
	messageCountCache map[int64]int64
	netmailCountCache int64
	countCacheValid   bool
)

// SQLArea implements AreaPrimitive interface for jnode SQL database
type SQLArea struct {
	db       *gorm.DB
	areaID   int64
	areaName string
	areaType EchoAreaType
	chrs     string

	// Cache for message list
	messageListCache []MessageListItem
	messageListValid bool

	// Last read tracking
	lastReadPosition uint32
}

// NewSQLArea creates a new SQL area instance
func NewSQLArea(db *gorm.DB, echoarea database.Echoarea) *SQLArea {
	area := &SQLArea{
		db:       db,
		areaID:   echoarea.ID,
		areaName: echoarea.Name,
		chrs:     "", // Will be set from configuration
	}

	// Map jnode area type to gossiped area type
	area.areaType = mapJnodeAreaType(echoarea.Name)

	return area
}

// NewSQLNetmailArea creates a new SQL netmail area instance
func NewSQLNetmailArea(db *gorm.DB) *SQLArea {
	return &SQLArea{
		db:       db,
		areaID:   0, // Netmail doesn't have echoarea_id
		areaName: "Netmail",
		areaType: EchoAreaTypeNetmail,
		chrs:     "",
	}
}

// mapJnodeAreaType maps area name to type (simple heuristic)
func mapJnodeAreaType(areaName string) EchoAreaType {
	// This is a simple mapping - in a real implementation,
	// you might want to store area type in the database
	switch areaName {
	case "Netmail":
		return EchoAreaTypeNetmail
	case "BadMail", "Bad":
		return EchoAreaTypeBad
	case "DupeMail", "Dupe":
		return EchoAreaTypeDupe
	default:
		// Most areas are echo areas
		return EchoAreaTypeEcho
	}
}

// Init initializes the area (required by AreaPrimitive interface)
func (a *SQLArea) Init() {
	// Load last read position if available
	// This could be stored in a separate table or user preferences
	a.lastReadPosition = 0
	a.messageListValid = false
}

// RefreshMessageCounts loads all message counts from database
func RefreshMessageCounts() error {
	counts, err := database.GetAllEchoareaCounts()
	if err != nil {
		return fmt.Errorf("failed to get echoarea counts: %w", err)
	}

	netmailCount, err := database.GetNetmailCount()
	if err != nil {
		return fmt.Errorf("failed to get netmail count: %w", err)
	}

	messageCountCache = counts
	netmailCountCache = netmailCount
	countCacheValid = true

	log.Printf("Loaded message counts for %d echoareas and %d netmail messages", len(counts), netmailCount)
	return nil
}

// InvalidateMessageCounts clears the message count cache
func InvalidateMessageCounts() {
	countCacheValid = false
	messageCountCache = nil
	netmailCountCache = 0
}

// IncrementMessageCount increments the cached count for a specific area
func IncrementMessageCount(areaID int64, isNetmail bool) {
	if !countCacheValid {
		return // No cache to update
	}

	if isNetmail {
		netmailCountCache++
	} else {
		if messageCountCache == nil {
			messageCountCache = make(map[int64]int64)
		}
		messageCountCache[areaID]++
	}
}

// GetCount returns the total number of messages in the area
func (a *SQLArea) GetCount() uint32 {
	// Use cached count if available
	if countCacheValid {
		if a.areaType == EchoAreaTypeNetmail {
			return uint32(netmailCountCache)
		} else {
			if count, exists := messageCountCache[a.areaID]; exists {
				return uint32(count)
			}
			return 0 // Area has no messages
		}
	}

	// Fallback to individual query if cache is not valid
	var count int64

	if a.areaType == EchoAreaTypeNetmail {
		// Count netmail messages
		if err := a.db.Model(&database.Netmail{}).Count(&count).Error; err != nil {
			log.Printf("Error counting netmail messages: %v", err)
			return 0
		}
	} else {
		// Count echomail messages for this area
		if err := a.db.Model(&database.Echomail{}).Where("echoarea_id = ?", a.areaID).Count(&count).Error; err != nil {
			log.Printf("Error counting echomail messages for area %s: %v", a.areaName, err)
			return 0
		}
	}

	return uint32(count)
}

// GetLast returns the last read message position
func (a *SQLArea) GetLast() uint32 {
	// First try to get from local SQLite database if enabled
	if database.IsLastReadEnabled() {
		position, err := database.GetLastRead(config.Config.Username, a.areaName)
		if err != nil {
			log.Printf("Error getting lastread from SQLite for area %s: %v", a.areaName, err)
			// Fall back to memory cache
			return a.lastReadPosition
		}
		return position
	}
	
	// Fall back to memory cache
	return a.lastReadPosition
}

// SetLast sets the last read message position
func (a *SQLArea) SetLast(position uint32) {
	// Update memory cache
	a.lastReadPosition = position
	
	// Save to local SQLite database if enabled
	if database.IsLastReadEnabled() {
		err := database.SetLastRead(config.Config.Username, a.areaName, position)
		if err != nil {
			log.Printf("Error saving lastread to SQLite for area %s: %v", a.areaName, err)
			// Don't fail the operation if lastread save fails
		}
	}
}

// GetMsg retrieves a message at the specified position
func (a *SQLArea) GetMsg(position uint32) (*Message, error) {
	if position == 0 {
		position = 1
	}

	if a.areaType == EchoAreaTypeNetmail {
		return a.getNetmailMessage(position)
	} else {
		return a.getEchomailMessage(position)
	}
}

// getEchomailMessage retrieves an echomail message
func (a *SQLArea) getEchomailMessage(position uint32) (*Message, error) {
	var echomail database.Echomail

	// Get message by position (offset)
	err := a.db.Where("echoarea_id = ?", a.areaID).
		Order("id ASC").
		Offset(int(position - 1)).
		Limit(1).
		First(&echomail).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error retrieving echomail message: %w", err)
	}

	// Convert database record to Message struct
	msg := &Message{
		Area:        a.areaName,
		AreaObject:  nil, // Will be set by caller if needed
		MsgNum:      position,
		MaxNum:      a.GetCount(),
		From:        echomail.FromName,
		To:          echomail.ToName,
		Subject:     echomail.Subject,
		Body:        a.NormalizeFromStorage(echomail.Message), // Convert \n to \r for FTN processing
		DateWritten: dateHelper.FromUnixTime(echomail.Date),
		DateArrived: dateHelper.FromUnixTime(echomail.Date),
		Attrs:       []string{}, // Parse attributes if needed
		Kludges:     make(map[string]string),
		Corrupted:   false,
	}

	// Parse FTN address
	msg.FromAddr = types.AddrFromString(echomail.FromFtnAddr)
	if msg.FromAddr == nil {
		msg.FromAddr = &types.FidoAddr{}
		msg.Corrupted = true
	}

	// For echomail, ToAddr is usually not meaningful
	msg.ToAddr = &types.FidoAddr{}

	// Parse message for kludges and other FTN-specific content (jnode SQL specific - no auto-decode)
	err = msg.ParseRawNoDecoding()
	if err != nil {
		log.Printf("Error parsing message %d: %v", position, err)
	}
	
	// For jnode SQL: Override charset behavior
	// Database always stores UTF-8, convert to display charset from config
	displayCharset := strings.Split(config.Config.Chrs.Default, " ")[0]
	if displayCharset != "UTF-8" {
		msg.Body = utils.EncodeCharmap(msg.Body, displayCharset)
		msg.From = utils.EncodeCharmap(msg.From, displayCharset)
		msg.To = utils.EncodeCharmap(msg.To, displayCharset)
		msg.Subject = utils.EncodeCharmap(msg.Subject, displayCharset)
	}

	return msg, nil
}

// getNetmailMessage retrieves a netmail message
func (a *SQLArea) getNetmailMessage(position uint32) (*Message, error) {
	var netmail database.Netmail

	// Get message by position (offset)
	err := a.db.Order("id ASC").
		Offset(int(position - 1)).
		Limit(1).
		First(&netmail).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error retrieving netmail message: %w", err)
	}

	// Convert database record to Message struct
	msg := &Message{
		Area:        a.areaName,
		AreaObject:  nil,
		MsgNum:      position,
		MaxNum:      a.GetCount(),
		From:        netmail.FromName,
		To:          netmail.ToName,
		Subject:     netmail.Subject,
		Body:        a.NormalizeFromStorage(netmail.Text), // Convert \n to \r for FTN processing
		DateWritten: dateHelper.FromUnixTime(netmail.Date),
		DateArrived: dateHelper.FromUnixTime(netmail.Date),
		Attrs:       a.parseNetmailAttrs(netmail.Attr),
		Kludges:     make(map[string]string),
		Corrupted:   false,
	}

	// Parse FTN addresses
	msg.FromAddr = types.AddrFromString(netmail.FromAddress)
	msg.ToAddr = types.AddrFromString(netmail.ToAddress)

	if msg.FromAddr == nil {
		msg.FromAddr = &types.FidoAddr{}
		msg.Corrupted = true
	}
	if msg.ToAddr == nil {
		msg.ToAddr = &types.FidoAddr{}
		msg.Corrupted = true
	}

	// Parse message for kludges (jnode SQL specific - no auto-decode)
	err = msg.ParseRawNoDecoding()
	if err != nil {
		log.Printf("Error parsing netmail %d: %v", position, err)
	}
	
	// For jnode SQL: Override charset behavior - same as echomail
	// Database always stores UTF-8, convert to display charset from config
	displayCharset := strings.Split(config.Config.Chrs.Default, " ")[0]
	if displayCharset != "UTF-8" {
		msg.Body = utils.EncodeCharmap(msg.Body, displayCharset)
		msg.From = utils.EncodeCharmap(msg.From, displayCharset)
		msg.To = utils.EncodeCharmap(msg.To, displayCharset)
		msg.Subject = utils.EncodeCharmap(msg.Subject, displayCharset)
	}

	return msg, nil
}

// parseNetmailAttrs converts jnode integer attributes to gossiped string attributes
func (a *SQLArea) parseNetmailAttrs(attr int) []string {
	var attrs []string

	// Map jnode netmail attributes to string representations
	// These constants should match jnode's NetmailAttrs
	if attr&256 != 0 { // MSG_LOCAL
		attrs = append(attrs, "Loc")
	}
	if attr&1 != 0 { // MSG_PRIVATE
		attrs = append(attrs, "Pvt")
	}
	if attr&2 != 0 { // MSG_CRASH
		attrs = append(attrs, "Cra")
	}
	if attr&4 != 0 { // MSG_READ
		attrs = append(attrs, "Rcv")
	}
	if attr&8 != 0 { // MSG_SENT
		attrs = append(attrs, "Snt")
	}
	if attr&16 != 0 { // MSG_FILE
		attrs = append(attrs, "Att")
	}
	if attr&32 != 0 { // MSG_FORWARD
		attrs = append(attrs, "Fwd")
	}
	if attr&128 != 0 { // MSG_KILL
		attrs = append(attrs, "K/s")
	}
	if attr&512 != 0 { // MSG_HOLD
		attrs = append(attrs, "Hld")
	}

	return attrs
}

// GetName returns the area name
func (a *SQLArea) GetName() string {
	return a.areaName
}

// GetMsgType returns the message base type
func (a *SQLArea) GetMsgType() EchoAreaMsgType {
	// For SQL areas, we use a custom type
	return EchoAreaMsgTypeSQL
}

// GetType returns the area type
func (a *SQLArea) GetType() EchoAreaType {
	return a.areaType
}

// SetChrs sets the character set for the area
func (a *SQLArea) SetChrs(chrs string) {
	a.chrs = chrs
}

// GetChrs returns the character set for the area
func (a *SQLArea) GetChrs() string {
	return a.chrs
}

// GetMessages returns a list of message headers
func (a *SQLArea) GetMessages() *[]MessageListItem {
	if a.messageListValid {
		return &a.messageListCache
	}

	// Clear cache and rebuild
	a.messageListCache = nil

	if a.areaType == EchoAreaTypeNetmail {
		a.loadNetmailList()
	} else {
		a.loadEchomailList()
	}

	a.messageListValid = true
	return &a.messageListCache
}

// loadEchomailList loads the message list for echomail
func (a *SQLArea) loadEchomailList() {
	var echomails []database.Echomail

	err := a.db.Where("echoarea_id = ?", a.areaID).
		Order("id ASC").
		Select("id", "from_name", "to_name", "subject", "date").
		Find(&echomails).Error

	if err != nil {
		log.Printf("Error loading echomail list for area %s: %v", a.areaName, err)
		return
	}

	for i, echomail := range echomails {
		item := MessageListItem{
			MsgNum:      uint32(i + 1),
			From:        echomail.FromName,
			To:          echomail.ToName,
			Subject:     echomail.Subject,
			DateWritten: dateHelper.FromUnixTime(echomail.Date),
		}
		a.messageListCache = append(a.messageListCache, item)
	}
}

// loadNetmailList loads the message list for netmail
func (a *SQLArea) loadNetmailList() {
	var netmails []database.Netmail

	err := a.db.Order("id ASC").
		Select("id", "from_name", "to_name", "subject", "date").
		Find(&netmails).Error

	if err != nil {
		log.Printf("Error loading netmail list: %v", err)
		return
	}

	for i, netmail := range netmails {
		item := MessageListItem{
			MsgNum:      uint32(i + 1),
			From:        netmail.FromName,
			To:          netmail.ToName,
			Subject:     netmail.Subject,
			DateWritten: dateHelper.FromUnixTime(netmail.Date),
		}
		a.messageListCache = append(a.messageListCache, item)
	}
}

// SaveMsg saves a new message to the database
func (a *SQLArea) SaveMsg(msg *Message) error {
	if a.areaType == EchoAreaTypeNetmail {
		return a.saveNetmailMessage(msg)
	} else {
		return a.saveEchomailMessage(msg)
	}
}

// saveEchomailMessage saves an echomail message
func (a *SQLArea) saveEchomailMessage(msg *Message) error {
	// Set area object for proper line ending handling
	var areaPtr AreaPrimitive = a
	msg.AreaObject = &areaPtr
	
	// Ensure message body is processed
	msg.MakeBody()
	
	// For jnode SQL: Override CHRS kludge with jnode_default if configured
	if config.Config.Chrs.JnodeDefault != "" {
		// Remove any existing CHRS kludge variants
		delete(msg.Kludges, "CHRS:")
		delete(msg.Kludges, "CHRS")
		// Set jnode default CHRS
		msg.Kludges["CHRS:"] = config.Config.Chrs.JnodeDefault
	}

	// Build message with kludges included in text (jnode style)
	messageText := ""
	for kl, v := range msg.Kludges {
		// Skip MSGID since it's stored in dedicated msgid field
		if kl != "MSGID:" {
			messageText += "\x01" + kl + " " + v + "\x0d"
		}
	}
	messageText += msg.Body

	echomail := database.Echomail{
		EchoareaID:  a.areaID,
		FromName:    msg.From,
		ToName:      msg.To,
		FromFtnAddr: msg.FromAddr.String(),
		Date:        dateHelper.ToUnixTime(msg.DateWritten),
		Subject:     msg.Subject,
		Message:     messageText,
		SeenBy:      "", // Will be filled by tosser
		Path:        "", // Will be filled by tosser
		MsgID:       msg.Kludges["MSGID:"],
	}

	err := a.db.Create(&echomail).Error
	if err != nil {
		return fmt.Errorf("error saving echomail message: %w", err)
	}

	// Queue message for all subscribed links
	if err := a.queueEchomailForSubscribers(echomail.ID); err != nil {
		log.Printf("Warning: Failed to queue echomail for subscribers: %v", err)
		// Don't fail the entire operation if queueing fails
	}

	// Invalidate message list cache
	a.messageListValid = false

	// Increment message count cache when new messages are added
	IncrementMessageCount(a.areaID, false)

	log.Printf("Saved echomail message to area %s", a.areaName)
	return nil
}

// queueEchomailForSubscribers queues echomail message for all subscribed links
func (a *SQLArea) queueEchomailForSubscribers(echomailID int64) error {
	// Get all subscribed links for this echoarea
	var subscriptions []database.Subscription
	err := a.db.Where("echoarea_id = ?", a.areaID).Find(&subscriptions).Error
	if err != nil {
		return fmt.Errorf("error getting subscriptions for area %s: %w", a.areaName, err)
	}

	// Create EchomailAwaiting entries for each subscribed link
	var awaitingEntries []database.EchomailAwaiting
	for _, subscription := range subscriptions {
		awaitingEntries = append(awaitingEntries, database.EchomailAwaiting{
			LinkID:     subscription.LinkID,
			EchomailID: echomailID,
		})
	}

	// Batch insert all awaiting entries
	if len(awaitingEntries) > 0 {
		err = a.db.Create(&awaitingEntries).Error
		if err != nil {
			return fmt.Errorf("error creating echomail awaiting entries: %w", err)
		}
		log.Printf("Queued echomail message %d for %d subscribed links in area %s",
			echomailID, len(awaitingEntries), a.areaName)
	}

	return nil
}

// saveNetmailMessage saves a netmail message
func (a *SQLArea) saveNetmailMessage(msg *Message) error {
	log.Printf("DEBUG: saveNetmailMessage called - ToAddr: %s (Zone:%d Net:%d Node:%d Point:%d)", 
		msg.ToAddr.String(), msg.ToAddr.GetZone(), msg.ToAddr.GetNet(), msg.ToAddr.GetNode(), msg.ToAddr.GetPoint())
	
	// Set area object for proper line ending handling
	var areaPtr AreaPrimitive = a
	msg.AreaObject = &areaPtr
	
	// Ensure message body is processed
	msg.MakeBody()
	
	// For jnode SQL: Override CHRS kludge with jnode_default if configured
	if config.Config.Chrs.JnodeDefault != "" {
		// Remove any existing CHRS kludge variants
		delete(msg.Kludges, "CHRS:")
		delete(msg.Kludges, "CHRS")
		// Set jnode default CHRS
		msg.Kludges["CHRS:"] = config.Config.Chrs.JnodeDefault
	}

	// Build message with kludges included in text (jnode style)
	messageText := ""
	for kl, v := range msg.Kludges {
		// Skip MSGID since it's stored in dedicated msgid field
		if kl != "MSGID:" {
			messageText += "\x01" + kl + " " + v + "\x0d"
		}
	}
	messageText += msg.Body

	// Convert attributes back to integer format
	attr := a.convertAttrsToInt(msg.Attrs)

	// Find routing for this netmail
	log.Printf("DEBUG: Before findNetmailRoute - ToAddr: %s (Zone:%d Net:%d Node:%d Point:%d)", 
		msg.ToAddr.String(), msg.ToAddr.GetZone(), msg.ToAddr.GetNet(), msg.ToAddr.GetNode(), msg.ToAddr.GetPoint())
	routeVia, err := a.findNetmailRoute(msg)
	if err != nil {
		log.Printf("Warning: Failed to find route for netmail: %v", err)
		// Continue without routing - might be handled later
	}

	netmail := database.Netmail{
		FromName:     msg.From,
		ToName:       msg.To,
		FromAddress:  msg.FromAddr.String(),
		ToAddress:    msg.ToAddr.String(),
		Subject:      msg.Subject,
		Text:         messageText,
		Date:         dateHelper.ToUnixTime(msg.DateWritten),
		Send:         false, // Always false for unsent mail (jnode will set to true after sending)
		Attr:         attr,
		LastModified: dateHelper.ToUnixTime(time.Now()),
		RouteVia:     routeVia, // This should be nil for direct routing or Link ID for routing via link
	}

	err = a.db.Create(&netmail).Error
	if err != nil {
		return fmt.Errorf("error saving netmail message: %w", err)
	}

	if routeVia != nil {
		log.Printf("Netmail queued for sending via link %d", *routeVia)
	} else {
		log.Printf("Netmail saved without route - manual routing may be needed")
	}

	// Invalidate message list cache
	a.messageListValid = false

	// Increment message count cache when new messages are added
	IncrementMessageCount(0, true)

	log.Printf("Saved netmail message")
	return nil
}

// findNetmailRoute implements complex netmail routing logic
func (a *SQLArea) findNetmailRoute(msg *Message) (*int64, error) {
	destAddr := msg.ToAddr.String()
	log.Printf("DEBUG: findNetmailRoute called for destination: %s", destAddr)
	log.Printf("DEBUG: ToAddr details - Zone:%d Net:%d Node:%d Point:%d", 
		msg.ToAddr.GetZone(), msg.ToAddr.GetNet(), msg.ToAddr.GetNode(), msg.ToAddr.GetPoint())

	// Step 1: Try direct link
	var link database.Link
	log.Printf("DEBUG: Step 1 - Looking for direct link to: %s", destAddr)
	err := a.db.Where("ftn_address = ?", destAddr).First(&link).Error
	if err == nil {
		log.Printf("Found direct link for %s: %s", destAddr, link.StationName)
		// For direct links, jnode uses route_via = null (direct routing)
		return nil, nil
	}
	log.Printf("DEBUG: Step 1 failed - %v", err)

	// Step 2: If not found, try without point
	if msg.ToAddr.GetPoint() != 0 {
		// Create address without point using existing FidoAddr functions
		addrWithoutPoint := types.AddrFromNum(
			msg.ToAddr.GetZone(),
			msg.ToAddr.GetNet(),
			msg.ToAddr.GetNode(),
			0, // point = 0
		).String()

		log.Printf("DEBUG: Step 2 - Looking for boss node: %s", addrWithoutPoint)
		err = a.db.Where("ftn_address = ?", addrWithoutPoint).First(&link).Error
		if err == nil {
			log.Printf("Found link without point for %s: %s", addrWithoutPoint, link.StationName)
			return &link.ID, nil
		}
		log.Printf("DEBUG: Step 2 failed - %v", err)
	}

	// Step 3: Process routing table
	var route database.Route
	err = a.db.Where(
		"(from_address = ? OR from_address = '*') AND "+
			"(to_address = ? OR to_address = '*') AND "+
			"(from_name = ? OR from_name = '*') AND "+
			"(to_name = ? OR to_name = '*') AND "+
			"(subject = ? OR subject = '*')",
		msg.FromAddr.String(), destAddr, msg.From, msg.To, msg.Subject).
		Order("nice ASC").
		First(&route).Error

	if err == nil {
		log.Printf("Found route via routing table for %s: link %d", destAddr, route.RouteVia)
		return &route.RouteVia, nil
	}

	return nil, fmt.Errorf("no route found for netmail to %s", destAddr)
}

// convertAttrsToInt converts string attributes back to integer format
func (a *SQLArea) convertAttrsToInt(attrs []string) int {
	var result int

	for _, attr := range attrs {
		switch attr {
		case "Loc":
			result |= 256 // MSG_LOCAL
		case "Pvt":
			result |= 1 // MSG_PRIVATE
		case "Cra":
			result |= 2 // MSG_CRASH
		case "Rcv":
			result |= 4 // MSG_READ
		case "Snt":
			result |= 8 // MSG_SENT
		case "Att":
			result |= 16 // MSG_FILE
		case "Fwd":
			result |= 32 // MSG_FORWARD
		case "K/s":
			result |= 128 // MSG_KILL
		case "Hld":
			result |= 512 // MSG_HOLD
		}
	}

	return result
}

// DelMsg deletes a message from the database
func (a *SQLArea) DelMsg(position uint32) error {
	if position == 0 {
		position = 1
	}

	if a.areaType == EchoAreaTypeNetmail {
		return a.deleteNetmailMessage(position)
	} else {
		return a.deleteEchomailMessage(position)
	}
}

// deleteEchomailMessage deletes an echomail message
func (a *SQLArea) deleteEchomailMessage(position uint32) error {
	var echomail database.Echomail

	// Find the message by position
	err := a.db.Where("echoarea_id = ?", a.areaID).
		Order("id ASC").
		Offset(int(position - 1)).
		Limit(1).
		First(&echomail).Error

	if err != nil {
		return fmt.Errorf("error finding echomail message to delete: %w", err)
	}

	// Delete the message
	err = a.db.Delete(&echomail).Error
	if err != nil {
		return fmt.Errorf("error deleting echomail message: %w", err)
	}

	// Invalidate message list cache
	a.messageListValid = false

	log.Printf("Deleted echomail message %d from area %s", position, a.areaName)
	return nil
}

// deleteNetmailMessage deletes a netmail message
func (a *SQLArea) deleteNetmailMessage(position uint32) error {
	var netmail database.Netmail

	// Find the message by position
	err := a.db.Order("id ASC").
		Offset(int(position - 1)).
		Limit(1).
		First(&netmail).Error

	if err != nil {
		return fmt.Errorf("error finding netmail message to delete: %w", err)
	}

	// Delete the message
	err = a.db.Delete(&netmail).Error
	if err != nil {
		return fmt.Errorf("error deleting netmail message: %w", err)
	}

	// Invalidate message list cache
	a.messageListValid = false

	log.Printf("Deleted netmail message %d", position)
	return nil
}

// Line ending handling methods for jnode SQL format
func (a *SQLArea) GetStorageLineEnding() string {
	return "\n" // jnode SQL stores Unix-style line endings
}

func (a *SQLArea) NormalizeForStorage(body string) string {
	// Convert FTN \r line endings to Unix \n for database storage
	// Remove any trailing \r to avoid double line endings
	result := strings.ReplaceAll(body, "\r", "\n")
	// Ensure single trailing newline for database consistency
	result = strings.TrimRight(result, "\n") + "\n"
	return result
}

func (a *SQLArea) NormalizeFromStorage(body string) string {
	// Convert Unix \n line endings from database to FTN \r for internal processing
	return strings.ReplaceAll(body, "\n", "\r")
}
